part of '../../main.dart';

class AppDebugRecorder {
  AppDebugRecorder._();

  static final AppDebugRecorder instance = AppDebugRecorder._();

  static const int _memoryLimit = 500;
  final Map<String, ListQueue<Map<String, dynamic>>> _buffers =
      <String, ListQueue<Map<String, dynamic>>>{};
  Future<void> _writeTail = Future<void>.value();

  Future<void> clearForTest() async {
    _buffers.clear();
    await _writeTail.catchError((_) {});
    if (kIsWeb) {
      return;
    }
    try {
      final dir = await getApplicationSupportDirectory();
      final traceDir = Directory(
        '${dir.path}${Platform.pathSeparator}debug_traces',
      );
      if (await traceDir.exists()) {
        await traceDir.delete(recursive: true);
      }
    } catch (_) {}
  }

  void record(
    String category, {
    String level = 'info',
    String source = 'flutter',
    String page = '',
    String action = '',
    String message = '',
    Map<String, dynamic>? data,
    Object? error,
    StackTrace? stack,
  }) {
    final normalizedCategory = _safeFileName(
      category.trim().isEmpty ? 'general' : category.trim().toLowerCase(),
    );
    final safePage = sanitizeWellFormedUtf16(page).trim();
    final safeAction = sanitizeWellFormedUtf16(action).trim();
    final safeMessage = sanitizeWellFormedUtf16(message).trim();
    final event = <String, dynamic>{
      'time': DateTime.now().toIso8601String(),
      'level': level,
      'category': normalizedCategory,
      'source': source,
      if (safePage.isNotEmpty) 'page': safePage,
      if (safeAction.isNotEmpty) 'action': safeAction,
      if (safeMessage.isNotEmpty) 'message': safeMessage,
      if (data != null && data.isNotEmpty) 'data': _jsonSafeMap(data),
      if (error != null) 'error': sanitizeWellFormedUtf16(error.toString()),
      if (stack != null) 'stack': _shortStack(stack),
    };
    final buffer = _buffers.putIfAbsent(
      normalizedCategory,
      () => ListQueue<Map<String, dynamic>>(),
    );
    buffer.addLast(event);
    while (buffer.length > _memoryLimit) {
      buffer.removeFirst();
    }
    _writeTail = _writeTail.catchError((_) {}).then((_) {
      return _appendEvent(normalizedCategory, event);
    });
  }

  void recordFlutterError(FlutterErrorDetails details) {
    record(
      'crash',
      level: 'error',
      source: 'FlutterError',
      message: details.exceptionAsString(),
      stack: details.stack,
      data: <String, dynamic>{
        if (details.library != null) 'library': details.library,
        if (details.context != null) 'context': details.context.toString(),
      },
    );
  }

  void recordPlatformError(Object error, StackTrace stack) {
    record(
      'crash',
      level: 'error',
      source: 'PlatformDispatcher',
      message: error.toString(),
      stack: stack,
    );
  }

  void recordVoskNativeEvent(Map<String, dynamic> event) {
    final type = (event['type'] ?? '').toString().trim();
    final payload = (event['payload'] ?? '').toString().trim();
    final parsed = _parseVoskPayload(payload);
    record(
      'voice_wake',
      source: 'vosk_method_channel',
      action: type.isEmpty ? 'wakeWordEvent' : type,
      message: parsed.text,
      data: <String, dynamic>{
        'engine': 'vosk',
        'event_type': type,
        'raw_payload': payload,
        'timestamp_native': event['timestamp'],
        'text': parsed.text,
        'alternatives': parsed.alternatives,
      },
    );
  }

  void recordVoiceWakeDecision({
    required String engine,
    required String eventType,
    required String rawPayload,
    required String transcript,
    required String wakePhrase,
    required String compactText,
    required bool matched,
    required String matchReason,
    required bool listening,
    required bool handling,
    List<String> alternatives = const <String>[],
    String command = '',
  }) {
    record(
      'voice_wake',
      source: engine,
      action: 'match_decision',
      message: transcript,
      data: <String, dynamic>{
        'engine': engine,
        'event_type': eventType,
        'raw_payload': rawPayload,
        'text': transcript,
        'alternatives': alternatives,
        'wake_phrase': wakePhrase,
        'compact_text': compactText,
        'matched': matched,
        'match_reason': matchReason,
        'listening': listening,
        'handling': handling,
        if (command.trim().isNotEmpty) 'command': command.trim(),
      },
    );
  }

  void recordMethodChannelError(
    String channel,
    String method,
    Object error,
    StackTrace stack,
  ) {
    record(
      'method_channel',
      level: 'error',
      source: channel,
      action: method,
      message: error.toString(),
      stack: stack,
    );
  }

  Future<List<Map<String, dynamic>>> recentEvents(
    String category, {
    int limit = 100,
  }) async {
    await _writeTail.catchError((_) {});
    final normalizedCategory = _safeFileName(category.trim().toLowerCase());
    final fileEvents = await _readTailEvents(normalizedCategory, limit: limit);
    if (fileEvents.isNotEmpty) {
      return fileEvents;
    }
    final memory = _buffers[normalizedCategory];
    if (memory == null || memory.isEmpty) {
      return const <Map<String, dynamic>>[];
    }
    return memory.toList(growable: false).reversed.take(limit).toList()
      ..sort((a, b) => '${a['time']}'.compareTo('${b['time']}'));
  }

  Future<void> _appendEvent(String category, Map<String, dynamic> event) async {
    if (kIsWeb) {
      return;
    }
    try {
      final file = await _traceFile(category);
      await file.parent.create(recursive: true);
      final sink = file.openWrite(mode: FileMode.append, encoding: utf8);
      sink.writeln(jsonEncode(event));
      await sink.close();
    } catch (err) {
      debugPrint('AppDebugRecorder write failed: $err');
    }
  }

  Future<List<Map<String, dynamic>>> _readTailEvents(
    String category, {
    required int limit,
  }) async {
    if (kIsWeb) {
      return const <Map<String, dynamic>>[];
    }
    try {
      final file = await _traceFile(category);
      if (!await file.exists()) {
        return const <Map<String, dynamic>>[];
      }
      final lines = await file.readAsLines(encoding: utf8);
      return lines.reversed
          .where((line) => line.trim().isNotEmpty)
          .take(limit)
          .map((line) {
            try {
              final decoded = jsonDecode(line);
              return decoded is Map<String, dynamic>
                  ? decoded
                  : <String, dynamic>{'raw': line};
            } catch (_) {
              return <String, dynamic>{'raw': line};
            }
          })
          .toList(growable: false)
          .reversed
          .toList(growable: false);
    } catch (err) {
      debugPrint('AppDebugRecorder read failed: $err');
      return const <Map<String, dynamic>>[];
    }
  }

  Future<File> _traceFile(String category) async {
    final dir = await getApplicationSupportDirectory();
    return File(
      '${dir.path}${Platform.pathSeparator}debug_traces'
      '${Platform.pathSeparator}${_safeFileName(category)}.jsonl',
    );
  }

  static String _safeFileName(String value) {
    final safe = value.replaceAll(RegExp(r'[^a-z0-9_\-]'), '_');
    return safe.isEmpty ? 'general' : safe;
  }

  static Map<String, dynamic> _jsonSafeMap(Map<String, dynamic> input) {
    return input.map((key, value) => MapEntry(key, _jsonSafeValue(value)));
  }

  static dynamic _jsonSafeValue(dynamic value) {
    if (value == null || value is num || value is bool || value is String) {
      return value is String ? sanitizeWellFormedUtf16(value) : value;
    }
    if (value is Iterable) {
      return value.map(_jsonSafeValue).toList(growable: false);
    }
    if (value is Map) {
      return value.map(
        (key, nested) => MapEntry(
          sanitizeWellFormedUtf16(key.toString()),
          _jsonSafeValue(nested),
        ),
      );
    }
    return sanitizeWellFormedUtf16(value.toString());
  }

  static String _shortStack(StackTrace stack) {
    return sanitizeWellFormedUtf16(
      stack.toString().split('\n').take(20).join('\n'),
    );
  }

  static _ParsedVoskPayload _parseVoskPayload(String raw) {
    if (raw.trim().isEmpty) {
      return const _ParsedVoskPayload(text: '', alternatives: <String>[]);
    }
    try {
      final decoded = jsonDecode(raw);
      if (decoded is Map) {
        final text = normalizeSpeechTranscript(
          (decoded['text'] ?? decoded['partial'] ?? '').toString(),
        );
        final alternatives = decoded['alternatives'];
        return _ParsedVoskPayload(
          text: text,
          alternatives: alternatives is List
              ? alternatives
                    .whereType<Map>()
                    .map(
                      (item) => normalizeSpeechTranscript(
                        (item['text'] ?? '').toString(),
                      ),
                    )
                    .where((item) => item.isNotEmpty)
                    .toList(growable: false)
              : const <String>[],
        );
      }
    } catch (_) {}
    return _ParsedVoskPayload(
      text: normalizeSpeechTranscript(raw),
      alternatives: const <String>[],
    );
  }
}

class _ParsedVoskPayload {
  const _ParsedVoskPayload({required this.text, required this.alternatives});

  final String text;
  final List<String> alternatives;
}
