part of '../../main.dart';

class SherpaWakeEvent {
  SherpaWakeEvent({
    required this.type,
    this.keyword = '',
    this.message = '',
    DateTime? timestamp,
  }) : timestamp = timestamp ?? DateTime.now();

  final String type;
  final String keyword;
  final String message;
  final DateTime timestamp;

  Map<String, dynamic> toJson() {
    return <String, dynamic>{
      'type': type,
      'keyword': keyword,
      'message': message,
      'timestamp': timestamp.millisecondsSinceEpoch,
    };
  }
}

class SherpaSpeechEngine {
  SherpaSpeechEngine();

  static bool _bindingsInitialized = false;

  final AudioRecorder _wakeRecorder = AudioRecorder();
  SherpaModelBundle? _bundle;
  SherpaKeywordTokenizer? _keywordTokenizer;
  sherpa.OfflineRecognizer? _asrRecognizer;
  sherpa.KeywordSpotter? _keywordSpotter;
  sherpa.OnlineStream? _keywordStream;
  StreamSubscription<Uint8List>? _wakeSubscription;
  Future<void> _wakeProcessing = Future<void>.value();
  bool _wakeListening = false;

  bool get isReady => _asrRecognizer != null && _bundle != null;
  bool get isWakeListening => _wakeListening;

  Future<String?> initialize(SherpaModelBundle bundle) async {
    try {
      _ensureBindingsInitialized();
      await stopWakeListening();
      _asrRecognizer?.free();
      _asrRecognizer = sherpa.OfflineRecognizer(
        sherpa.OfflineRecognizerConfig(
          model: sherpa.OfflineModelConfig(
            senseVoice: sherpa.OfflineSenseVoiceModelConfig(
              model: bundle.asr.modelPath,
              language: 'zh',
              useInverseTextNormalization: true,
            ),
            tokens: bundle.asr.tokensPath,
            numThreads: 2,
            debug: false,
            provider: 'cpu',
          ),
        ),
      );
      _keywordTokenizer = await SherpaKeywordTokenizer.fromTokensFile(
        bundle.kws.tokensPath,
      );
      _bundle = bundle;
      return null;
    } catch (err) {
      _bundle = null;
      _keywordTokenizer = null;
      _asrRecognizer?.free();
      _asrRecognizer = null;
      return 'Sherpa initialize failed: $err';
    }
  }

  Future<String> transcribeFile(String audioPath) async {
    final recognizer = _asrRecognizer;
    if (recognizer == null) {
      throw StateError('Sherpa ASR model is not initialized');
    }
    final wave = sherpa.readWave(audioPath);
    if (wave.samples.isEmpty || wave.sampleRate <= 0) {
      return '';
    }
    final stream = recognizer.createStream();
    try {
      stream.acceptWaveform(samples: wave.samples, sampleRate: wave.sampleRate);
      recognizer.decode(stream);
      return recognizer.getResult(stream).text.trim();
    } finally {
      stream.free();
    }
  }

  Future<bool> startWakeListening({
    required Iterable<String> wakePhrases,
    required Future<void> Function(SherpaWakeEvent event) onEvent,
  }) async {
    if (_wakeListening) {
      return true;
    }
    final bundle = _bundle;
    final tokenizer = _keywordTokenizer;
    if (bundle == null || tokenizer == null) {
      await onEvent(
        SherpaWakeEvent(type: 'error', message: 'Sherpa model not ready'),
      );
      return false;
    }
    final hasPermission = await _wakeRecorder.hasPermission();
    if (!hasPermission) {
      await onEvent(
        SherpaWakeEvent(type: 'error', message: 'Microphone denied'),
      );
      return false;
    }

    try {
      final keywords = tokenizer.buildKeywordBuffer(wakePhrases);
      _recreateKeywordSpotter(bundle, keywords.keywordBuffer);
      _keywordStream = _keywordSpotter!.createStream();
      final audioStream = await _wakeRecorder.startStream(
        const RecordConfig(
          encoder: AudioEncoder.pcm16bits,
          sampleRate: 16000,
          numChannels: 1,
          streamBufferSize: 3200,
        ),
      );
      _wakeListening = true;
      _wakeSubscription = audioStream.listen(
        (chunk) {
          _wakeProcessing = _wakeProcessing
              .then((_) => _processWakeChunk(chunk, onEvent))
              .catchError((Object err, StackTrace stack) async {
                await _emitWakeError(onEvent, err, stack);
              });
        },
        onError: (Object err, StackTrace stack) async {
          await _emitWakeError(onEvent, err, stack);
        },
        cancelOnError: false,
      );
      return true;
    } catch (err, stack) {
      await _emitWakeError(onEvent, err, stack);
      return false;
    }
  }

  Future<void> stopWakeListening() async {
    _wakeListening = false;
    final sub = _wakeSubscription;
    _wakeSubscription = null;
    if (sub != null) {
      await sub.cancel();
    }
    try {
      await _wakeRecorder.stop();
    } catch (_) {}
    try {
      await _wakeProcessing.catchError((_) {});
    } catch (_) {}
    _freeKeywordRuntime();
  }

  Future<String> readNativeDebugTrace(String category) async {
    return '';
  }

  Future<void> dispose() async {
    await stopWakeListening();
    _asrRecognizer?.free();
    _asrRecognizer = null;
    await _wakeRecorder.dispose();
  }

  void _recreateKeywordSpotter(SherpaModelBundle bundle, String keywordBuffer) {
    _freeKeywordRuntime();
    _keywordSpotter = sherpa.KeywordSpotter(
      sherpa.KeywordSpotterConfig(
        model: sherpa.OnlineModelConfig(
          transducer: sherpa.OnlineTransducerModelConfig(
            encoder: bundle.kws.encoderPath,
            decoder: bundle.kws.decoderPath,
            joiner: bundle.kws.joinerPath,
          ),
          tokens: bundle.kws.tokensPath,
          numThreads: 1,
          provider: 'cpu',
          debug: false,
        ),
        keywordsBuf: keywordBuffer,
        keywordsBufSize: utf8.encode(keywordBuffer).length,
        keywordsScore: 2.0,
        keywordsThreshold: 0.25,
      ),
    );
  }

  Future<void> _processWakeChunk(
    Uint8List chunk,
    Future<void> Function(SherpaWakeEvent event) onEvent,
  ) async {
    if (!_wakeListening || chunk.length < 2) {
      return;
    }
    final spotter = _keywordSpotter;
    final stream = _keywordStream;
    if (spotter == null || stream == null) {
      return;
    }
    final samples = _pcm16LittleEndianToFloat32(chunk);
    if (samples.isEmpty) {
      return;
    }
    stream.acceptWaveform(samples: samples, sampleRate: 16000);
    while (_wakeListening && spotter.isReady(stream)) {
      spotter.decode(stream);
      final result = spotter.getResult(stream);
      final keyword = result.keyword.trim();
      if (keyword.isEmpty) {
        continue;
      }
      await onEvent(SherpaWakeEvent(type: 'keyword', keyword: keyword));
      if (_wakeListening) {
        _resetKeywordStream();
      }
      return;
    }
  }

  Future<void> _emitWakeError(
    Future<void> Function(SherpaWakeEvent event) onEvent,
    Object err,
    StackTrace stack,
  ) async {
    _wakeListening = false;
    final sub = _wakeSubscription;
    _wakeSubscription = null;
    if (sub != null) {
      await sub.cancel();
    }
    try {
      await _wakeRecorder.stop();
    } catch (_) {}
    _freeKeywordRuntime();
    AppDebugRecorder.instance.recordPlatformError(err, stack);
    await onEvent(SherpaWakeEvent(type: 'error', message: err.toString()));
  }

  void _resetKeywordStream() {
    try {
      _keywordStream?.free();
    } catch (_) {}
    _keywordStream = _keywordSpotter?.createStream();
  }

  void _freeKeywordRuntime() {
    try {
      _keywordStream?.free();
    } catch (_) {}
    _keywordStream = null;
    try {
      _keywordSpotter?.free();
    } catch (_) {}
    _keywordSpotter = null;
  }

  static Float32List _pcm16LittleEndianToFloat32(Uint8List bytes) {
    final sampleCount = bytes.length ~/ 2;
    if (sampleCount <= 0) {
      return Float32List(0);
    }
    final data = ByteData.sublistView(bytes, 0, sampleCount * 2);
    final samples = Float32List(sampleCount);
    for (var i = 0; i < sampleCount; i++) {
      samples[i] = data.getInt16(i * 2, Endian.little) / 32768.0;
    }
    return samples;
  }

  static void _ensureBindingsInitialized() {
    if (_bindingsInitialized) {
      return;
    }
    sherpa.initBindings();
    _bindingsInitialized = true;
  }
}
