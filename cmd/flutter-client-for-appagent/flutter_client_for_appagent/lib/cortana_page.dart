import 'dart:async';
import 'dart:collection';
import 'dart:convert';
import 'dart:io';
import 'dart:math' as math;
import 'package:audioplayers/audioplayers.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_inappwebview/flutter_inappwebview.dart';
import 'package:path_provider/path_provider.dart';

class FlutterClientLogEntry {
  const FlutterClientLogEntry({required this.timestamp, required this.message});
  final DateTime timestamp;
  final String message;

  String get timeLabel {
    final hh = timestamp.hour.toString().padLeft(2, '0');
    final mm = timestamp.minute.toString().padLeft(2, '0');
    final ss = timestamp.second.toString().padLeft(2, '0');
    return '$hh:$mm:$ss';
  }
}

final List<FlutterClientLogEntry> flutterClientLogs = <FlutterClientLogEntry>[];
const int _maxFlutterClientLogs = 200;

void addFlutterClientLog(String message) {
  final text = message.trim();
  if (text.isEmpty) return;
  final entry = FlutterClientLogEntry(timestamp: DateTime.now(), message: text);
  flutterClientLogs.insert(0, entry);
  if (flutterClientLogs.length > _maxFlutterClientLogs) {
    flutterClientLogs.removeRange(
      _maxFlutterClientLogs,
      flutterClientLogs.length,
    );
  }
}

enum CortanaDisplayMode { fullscreen, expanded, small, collapsed }

class CortanaSettings {
  const CortanaSettings({
    this.enabled = true,
    this.allowFullAccess = true,
    this.autoPlay = true,
    this.proactiveMode = 'high',
    this.highFreqStartHour = 9,
    this.highFreqStartMinute = 0,
    this.highFreqEndHour = 22,
    this.highFreqEndMinute = 0,
    this.personaName = 'Cortana',
    this.ownerTitle = '',
    this.personaDescription = '',
    this.voiceWakeEnabled = false,
    this.wakePhrase = '嗨 Cortana',
  });

  final bool enabled;
  final bool allowFullAccess;
  final bool autoPlay;
  final String proactiveMode;
  final int highFreqStartHour;
  final int highFreqStartMinute;
  final int highFreqEndHour;
  final int highFreqEndMinute;
  final String personaName;
  final String ownerTitle;
  final String personaDescription;
  final bool voiceWakeEnabled;
  final String wakePhrase;

  CortanaSettings copyWith({
    bool? enabled,
    bool? allowFullAccess,
    bool? autoPlay,
    String? proactiveMode,
    int? highFreqStartHour,
    int? highFreqStartMinute,
    int? highFreqEndHour,
    int? highFreqEndMinute,
    String? personaName,
    String? ownerTitle,
    String? personaDescription,
    bool? voiceWakeEnabled,
    String? wakePhrase,
  }) {
    return CortanaSettings(
      enabled: enabled ?? this.enabled,
      allowFullAccess: allowFullAccess ?? this.allowFullAccess,
      autoPlay: autoPlay ?? this.autoPlay,
      proactiveMode: proactiveMode ?? this.proactiveMode,
      highFreqStartHour: highFreqStartHour ?? this.highFreqStartHour,
      highFreqStartMinute: highFreqStartMinute ?? this.highFreqStartMinute,
      highFreqEndHour: highFreqEndHour ?? this.highFreqEndHour,
      highFreqEndMinute: highFreqEndMinute ?? this.highFreqEndMinute,
      personaName: personaName ?? this.personaName,
      ownerTitle: ownerTitle ?? this.ownerTitle,
      personaDescription: personaDescription ?? this.personaDescription,
      voiceWakeEnabled: voiceWakeEnabled ?? this.voiceWakeEnabled,
      wakePhrase: wakePhrase ?? this.wakePhrase,
    );
  }

  Map<String, dynamic> toJson() {
    return <String, dynamic>{
      'enabled': enabled,
      'allow_full_access': allowFullAccess,
      'auto_play': autoPlay,
      'proactive_mode': proactiveMode,
      'high_freq_start_hour': highFreqStartHour,
      'high_freq_start_minute': highFreqStartMinute,
      'high_freq_end_hour': highFreqEndHour,
      'high_freq_end_minute': highFreqEndMinute,
      'persona_name': personaName,
      'owner_title': ownerTitle,
      'persona_description': personaDescription,
      'voice_wake_enabled': voiceWakeEnabled,
      'wake_phrase': wakePhrase,
    };
  }

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) return true;
    return other is CortanaSettings &&
        other.enabled == enabled &&
        other.allowFullAccess == allowFullAccess &&
        other.autoPlay == autoPlay &&
        other.proactiveMode == proactiveMode &&
        other.highFreqStartHour == highFreqStartHour &&
        other.highFreqStartMinute == highFreqStartMinute &&
        other.highFreqEndHour == highFreqEndHour &&
        other.highFreqEndMinute == highFreqEndMinute &&
        other.personaName == personaName &&
        other.ownerTitle == ownerTitle &&
        other.personaDescription == personaDescription &&
        other.voiceWakeEnabled == voiceWakeEnabled &&
        other.wakePhrase == wakePhrase;
  }

  @override
  int get hashCode => Object.hash(
    enabled,
    allowFullAccess,
    autoPlay,
    proactiveMode,
    highFreqStartHour,
    highFreqStartMinute,
    highFreqEndHour,
    highFreqEndMinute,
    personaName,
    ownerTitle,
    personaDescription,
    voiceWakeEnabled,
    wakePhrase,
  );
}

class CortanaReplyPayload {
  const CortanaReplyPayload({
    required this.text,
    this.audioPath = '',
    this.audioBytes,
    this.audioFormat = '',
    this.actionPlan,
    this.requestId = '',
  });

  final String text;
  final String audioPath;
  final Uint8List? audioBytes;
  final String audioFormat;
  final Map<String, dynamic>? actionPlan;
  final String requestId;

  bool get hasAudio => audioPath.trim().isNotEmpty || audioBytes != null;
}

class CortanaReplayItem {
  const CortanaReplayItem({
    required this.id,
    required this.text,
    required this.audioPath,
    this.audioBytes,
    required this.audioFormat,
    required this.createdAt,
    this.actionPlan,
    this.sourceLabel = '',
    this.fileId = '',
    this.storageProvider = '',
    this.objectKey = '',
  });

  final String id;
  final String text;
  final String audioPath;
  final Uint8List? audioBytes;
  final String audioFormat;
  final DateTime createdAt;
  final Map<String, dynamic>? actionPlan;
  final String sourceLabel;
  final String fileId;
  final String storageProvider;
  final String objectKey;
}

class CortanaLogSource {
  const CortanaLogSource({
    required this.name,
    required this.path,
    required this.description,
  });

  final String name;
  final String path;
  final String description;
}

class CortanaLogFile {
  const CortanaLogFile({
    required this.name,
    required this.path,
    required this.size,
    required this.modifiedAt,
    required this.modifiedText,
  });

  final String name;
  final String path;
  final int size;
  final DateTime? modifiedAt;
  final String modifiedText;
}

class _CortanaVoiceHistoryItem {
  const _CortanaVoiceHistoryItem({
    required this.id,
    required this.text,
    required this.audioPath,
    this.audioBytes,
    required this.audioFormat,
    required this.createdAt,
    this.actionPlan,
  });

  final String id;
  final String text;
  final String audioPath;
  final Uint8List? audioBytes;
  final String audioFormat;
  final DateTime createdAt;
  final Map<String, dynamic>? actionPlan;
}

class _QueuedBroadcast {
  const _QueuedBroadcast(this.payload, this.onFinished);

  final CortanaReplyPayload payload;
  final VoidCallback? onFinished;
}

class _CustomCortanaWebRoot {
  const _CustomCortanaWebRoot({required this.root, required this.modelUrl});

  final Directory root;
  final String modelUrl;
}

class CortanaPage extends StatefulWidget {
  const CortanaPage({
    super.key,
    this.mode = CortanaDisplayMode.fullscreen,
    this.onSendMessage,
    this.externalVoiceHistory = const <CortanaReplayItem>[],
    this.onTapWhenFloating,
    this.onLongPressWhenFloating,
    this.onModeChanged,
    this.contextualExpression,
    this.showBadge = false,
    this.autoCollapseDelay = const Duration(seconds: 8),
    this.floatingBottomInset = 0,
    this.onBroadcast,
    this.settings = const CortanaSettings(),
    this.onSettingsChanged,
    this.live2dModelUrl = '',
  });

  final CortanaDisplayMode mode;
  final Future<CortanaReplyPayload> Function(String message)? onSendMessage;
  final List<CortanaReplayItem> externalVoiceHistory;
  final VoidCallback? onTapWhenFloating;
  final VoidCallback? onLongPressWhenFloating;
  final ValueChanged<CortanaDisplayMode>? onModeChanged;
  final String? contextualExpression;
  final bool showBadge;
  final Duration autoCollapseDelay;
  final double floatingBottomInset;
  final void Function(CortanaReplyPayload payload)? onBroadcast;
  final CortanaSettings settings;
  final ValueChanged<CortanaSettings>? onSettingsChanged;
  final String live2dModelUrl;

  @override
  State<CortanaPage> createState() => CortanaPageState();
}

class CortanaPageState extends State<CortanaPage> {
  static const _jsLogHandlerName = 'cortanaLog';
  static const _cortanaHtmlAsset = 'assets/cortana/index.html';
  static const _cortanaLocalPath = 'index.html';
  static const _localhostPort = 18080;
  static const List<String> _webRuntimeAssets = <String>[
    'assets/cortana/index.html',
    'assets/cortana/vendor/pixi.min.js',
    'assets/cortana/vendor/live2dcubismcore.min.js',
    'assets/cortana/vendor/cubism4.min.js',
  ];
  InAppWebViewController? _webCtrl;
  final ListQueue<_QueuedBroadcast> _queuedBroadcasts =
      ListQueue<_QueuedBroadcast>();
  final AudioPlayer _audio = AudioPlayer();
  Timer? _lipTimer;
  Timer? _debugStateTimer;
  Timer? _broadcastAutoCollapseTimer;
  StreamSubscription<Duration>? _audioPositionSub;
  final List<Timer> _motionTimers = <Timer>[];
  bool _speaking = false;
  InAppLocalhostServer? _localhostServer;
  Future<void>? _androidLocalhostFuture;
  HttpServer? _customLocalhostServer;
  Future<void>? _customLocalhostFuture;
  int _playbackToken = 0;
  final List<_CortanaVoiceHistoryItem> _voiceHistory =
      <_CortanaVoiceHistoryItem>[];
  double _modelUserScale = 1.0;
  double _modelUserOffsetX = 0.0;
  double _modelUserOffsetY = 0.0;
  bool _controlPanelVisible = false;
  bool _live2dSummaryExpanded = false;
  bool _expressionActionsExpanded = false;
  bool _viewControlsExpanded = false;
  bool _replayExpanded = false;
  Map<String, dynamic>? _live2dDebugState;
  final List<String> _logEntries = <String>[];
  Offset? _floatingOffset;
  bool _isDragging = false;
  bool _webViewReady = false;
  String? _lastContextualExpression;
  Future<_CustomCortanaWebRoot>? _customWebRootFuture;
  int _webViewRevision = 0;

  bool get isSpeaking => _speaking;

  static const int _maxLogEntries = 80;
  static const double _floatingCollapsedSize = 48.0;
  static const double _floatingSmallWidth = 120.0;
  static const double _floatingSmallHeight = 120.0;
  static const double _floatingExpandedWidth = 240.0;
  static const double _floatingExpandedHeight = 320.0;

  static const _expressions = ['happy', 'sad', 'surprised'];
  static const _motions = ['Idle', 'IdleAlt', 'IdleWave', 'Tap'];
  static const Map<String, String> _expressionAliases = <String, String>{
    'happy': 'happy',
    'joy': 'happy',
    'smile': 'happy',
    'sad': 'sad',
    'sorry': 'sad',
    'apology': 'sad',
    'surprised': 'surprised',
    'excited': 'surprised',
    'wow': 'surprised',
    'alert': 'surprised',
  };
  static const Map<String, String> _motionAliases = <String, String>{
    'Idle': 'Idle',
    'IdleAlt': 'IdleAlt',
    'IdleWave': 'IdleWave',
    'Tap': 'Tap',
    'TapBody': 'Tap',
    'Greeting': 'IdleWave',
    'Explain': 'IdleAlt',
    'Emphasis': 'Tap',
    'Listen': 'Idle',
    'Thinking': 'IdleAlt',
    'ExplainCalm': 'Idle',
    'ExplainStrong': 'IdleAlt',
    'Celebrate': 'IdleWave',
  };

  @override
  void initState() {
    super.initState();
    if (!kIsWeb && defaultTargetPlatform == TargetPlatform.android) {
      if (_usesCustomLive2dModel) {
        _customWebRootFuture = _prepareCustomWebRoot();
      } else {
        _ensureAssetLocalhostServer();
      }
    }
    _debugStateTimer = Timer.periodic(
      const Duration(seconds: 3),
      (_) => unawaited(_refreshLive2dDebugState()),
    );
    if (_usesCustomLive2dModel && _customWebRootFuture == null) {
      _customWebRootFuture = _prepareCustomWebRoot();
    }
  }

  @override
  void didUpdateWidget(covariant CortanaPage oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (widget.contextualExpression != null &&
        widget.contextualExpression != _lastContextualExpression) {
      _lastContextualExpression = widget.contextualExpression;
      final expr = _normalizeExpression(widget.contextualExpression!);
      unawaited(_callJS("window.setExpression('$expr')"));
    }
    if (widget.settings != oldWidget.settings) {
      // Settings are now edited in the chat page controls;
      // CortanaPage only consumes the settings values.
    }
    if (widget.live2dModelUrl != oldWidget.live2dModelUrl) {
      final oldModelUrl = oldWidget.live2dModelUrl.trim().isEmpty
          ? 'default'
          : oldWidget.live2dModelUrl.trim();
      final newModelUrl = widget.live2dModelUrl.trim().isEmpty
          ? 'default'
          : widget.live2dModelUrl.trim();
      _appendLog(
        'Live2D model URL changed: old=$oldModelUrl new=$newModelUrl',
      );
      _webViewReady = false;
      _webCtrl = null;
      if (!kIsWeb && defaultTargetPlatform == TargetPlatform.android) {
        if (_usesCustomLive2dModel) {
          _customWebRootFuture = _prepareCustomWebRootAfterClosingAssetServer();
        } else {
          _customWebRootFuture = null;
          _androidLocalhostFuture =
              _restartAssetLocalhostServerAfterClosingCustomServer();
        }
      } else {
        _customWebRootFuture = _usesCustomLive2dModel
            ? _prepareCustomWebRoot()
            : null;
      }
      setState(() {
        _live2dDebugState = null;
        _webViewRevision++;
      });
    }
  }

  @override
  void dispose() {
    _resetPlaybackEffects();
    _debugStateTimer?.cancel();
    _broadcastAutoCollapseTimer?.cancel();
    _audio.dispose();
    final localhostServer = _localhostServer;
    if (localhostServer != null) {
      unawaited(localhostServer.close());
    }
    final customLocalhostServer = _customLocalhostServer;
    if (customLocalhostServer != null) {
      unawaited(customLocalhostServer.close(force: true));
    }
    super.dispose();
  }

  void _ensureAssetLocalhostServer() {
    if (_localhostServer != null || _androidLocalhostFuture != null) {
      return;
    }
    _localhostServer = InAppLocalhostServer(
      documentRoot: 'assets/cortana',
      port: _localhostPort,
    );
    _appendLog('Starting localhost server on $_localhostPort');
    _androidLocalhostFuture = _localhostServer!
        .start()
        .then((_) {
          if (mounted) {
            _updateLoadStatus(
              'Localhost ready: http://localhost:$_localhostPort/$_cortanaLocalPath',
            );
          }
        })
        .catchError((Object error) {
          if (mounted) {
            _updateLoadStatus('Localhost start failed: $error');
          }
          throw error;
        });
  }

  Future<_CustomCortanaWebRoot>
      _prepareCustomWebRootAfterClosingAssetServer() async {
    final localhostServer = _localhostServer;
    _localhostServer = null;
    _androidLocalhostFuture = null;
    if (localhostServer != null) {
      await localhostServer.close();
    }
    return _prepareCustomWebRoot();
  }

  Future<void> _restartAssetLocalhostServerAfterClosingCustomServer() async {
    final customLocalhostServer = _customLocalhostServer;
    _customLocalhostServer = null;
    _customLocalhostFuture = null;
    if (customLocalhostServer != null) {
      await customLocalhostServer.close(force: true);
    }
    _androidLocalhostFuture = null;
    _ensureAssetLocalhostServer();
    await _androidLocalhostFuture;
  }

  void _appendLog(String message) {
    final text = message.trim();
    if (text.isEmpty) {
      return;
    }
    // Forward to shared Flutter client log buffer
    addFlutterClientLog(text);
    final now = DateTime.now();
    final hh = now.hour.toString().padLeft(2, '0');
    final mm = now.minute.toString().padLeft(2, '0');
    final ss = now.second.toString().padLeft(2, '0');
    final entry = '[$hh:$mm:$ss] $text';
    debugPrint('[Cortana Log] $entry');
    if (!mounted) {
      _logEntries.insert(0, entry);
      if (_logEntries.length > _maxLogEntries) {
        _logEntries.removeRange(_maxLogEntries, _logEntries.length);
      }
      return;
    }
    setState(() {
      _logEntries.insert(0, entry);
      if (_logEntries.length > _maxLogEntries) {
        _logEntries.removeRange(_maxLogEntries, _logEntries.length);
      }
    });
  }

  void _updateLoadStatus(String message) {
    _appendLog(message);
  }

  bool _shouldSurfaceCortanaWebLog(String message) {
    if (message.trim().isEmpty) {
      return false;
    }
    const noisyFragments = <String>[
      'Model laid out:',
      'setMotion:',
      'Motion applied:',
      'setExpression:',
      'Expression applied:',
    ];
    if (noisyFragments.any(message.contains)) {
      return false;
    }
    const importantFragments = <String>[
      '[Cortana]',
      '[Cortana Stack]',
      'FETCH ',
      'FETCH ERROR',
      'MODEL_URL=',
      'Resolved model URL:',
      'Model resources discovered:',
      'Missing Cortana assets:',
      'Loading Live2D model',
      'Live2D model loaded',
      'Error:',
      'Window error:',
      'Unhandled rejection:',
    ];
    return importantFragments.any(message.contains);
  }

  bool get _usesCustomLive2dModel => widget.live2dModelUrl.trim().isNotEmpty;

  String get _effectiveLive2dModelUrl => widget.live2dModelUrl.trim();

  String _htmlWithModelOverride(String html, String modelUrl) {
    if (modelUrl.isEmpty) {
      return html;
    }
    _appendLog('Preparing Cortana custom model override: $modelUrl');
    final overrideScript =
        '<script>window.CORTANA_MODEL_URL=${jsonEncode(modelUrl)};</script>';
    return html.replaceFirst(
      '<script src="./vendor/pixi.min.js"></script>',
      '$overrideScript\n<script src="./vendor/pixi.min.js"></script>',
    );
  }

  Future<_CustomCortanaWebRoot> _prepareCustomWebRoot() async {
    final supportDir = await getApplicationSupportDirectory();
    final root = Directory(
      '${supportDir.path}${Platform.pathSeparator}cortana_web_runtime',
    );
    final modelUrl = await _prepareCustomModelForWebRoot(root);
    _appendLog(
      'Preparing Cortana web runtime: root=${root.path}, model=$modelUrl',
    );
    await root.create(recursive: true);
    for (final asset in _webRuntimeAssets) {
      final relative = asset.replaceFirst('assets/cortana/', '');
      final outFile = File('${root.path}${Platform.pathSeparator}$relative');
      await outFile.parent.create(recursive: true);
      if (asset.endsWith('.html')) {
        final html = await rootBundle.loadString(asset);
        await outFile.writeAsString(
          _htmlWithModelOverride(html, modelUrl),
          encoding: utf8,
        );
        _appendLog('Wrote Cortana runtime html: ${outFile.path}');
      } else {
        final data = await rootBundle.load(asset);
        await outFile.writeAsBytes(
          data.buffer.asUint8List(data.offsetInBytes, data.lengthInBytes),
        );
        _appendLog('Wrote Cortana runtime asset: ${outFile.path}');
      }
    }
    if (!kIsWeb && defaultTargetPlatform == TargetPlatform.android) {
      _ensureCustomLocalhostServer(root);
      await _customLocalhostFuture;
    }
    return _CustomCortanaWebRoot(root: root, modelUrl: modelUrl);
  }

  Future<String> _prepareCustomModelForWebRoot(Directory root) async {
    final modelUrl = _effectiveLive2dModelUrl;
    final uri = Uri.tryParse(modelUrl);
    if (uri == null || !uri.isScheme('file')) {
      return modelUrl;
    }

    final sourceModelJson = File(uri.toFilePath());
    if (!await sourceModelJson.exists()) {
      return modelUrl;
    }

    final outDir = Directory(
      '${root.path}${Platform.pathSeparator}custom_model',
    );
    await _deleteDirectoryIfExists(outDir);
    await outDir.create(recursive: true);
    await _copyDirectory(sourceModelJson.parent, outDir);
    final relativeModelPath = Uri(
      pathSegments: <String>[
        'custom_model',
        sourceModelJson.uri.pathSegments.last,
      ],
    ).toString();
    return './$relativeModelPath';
  }

  Future<void> _copyDirectory(Directory source, Directory destination) async {
    await for (final entity in source.list(
      recursive: false,
      followLinks: false,
    )) {
      final name = entity.uri.pathSegments.isEmpty
          ? ''
          : entity.uri.pathSegments.last;
      if (name.isEmpty) {
        continue;
      }
      final outPath = '${destination.path}${Platform.pathSeparator}$name';
      if (entity is Directory) {
        final outDir = Directory(outPath);
        await outDir.create(recursive: true);
        await _copyDirectory(entity, outDir);
      } else if (entity is File) {
        await File(outPath).writeAsBytes(await entity.readAsBytes());
      }
    }
  }

  Future<void> _deleteDirectoryIfExists(Directory directory) async {
    if (await directory.exists()) {
      await directory.delete(recursive: true);
    }
  }

  void _ensureCustomLocalhostServer(Directory root) {
    if (_customLocalhostServer != null || _customLocalhostFuture != null) {
      return;
    }
    _customLocalhostFuture = HttpServer.bind(
      InternetAddress.loopbackIPv4,
      _localhostPort,
    )
        .then((server) {
          _customLocalhostServer = server;
          server.listen(
            (request) => unawaited(_serveCustomLocalhostFile(root, request)),
          );
          if (mounted) {
            _updateLoadStatus(
              'Custom localhost ready: http://localhost:$_localhostPort/$_cortanaLocalPath',
            );
          }
        })
        .catchError((Object error) {
          if (mounted) {
            _updateLoadStatus('Custom localhost start failed: $error');
          }
          throw error;
        });
  }

  Future<void> _serveCustomLocalhostFile(
    Directory root,
    HttpRequest request,
  ) async {
    try {
      if (request.method != 'GET' && request.method != 'HEAD') {
        request.response.statusCode = HttpStatus.methodNotAllowed;
        await request.response.close();
        return;
      }

      final segments = request.uri.pathSegments;
      if (segments.any((segment) => segment == '..')) {
        request.response.statusCode = HttpStatus.forbidden;
        await request.response.close();
        return;
      }
      final relativeSegments = segments.isEmpty
          ? <String>['index.html']
          : segments;
      final fileUri = root.absolute.uri.resolve(
        Uri(pathSegments: relativeSegments).toString(),
      );
      final rootPath = root.absolute.uri.toFilePath();
      final filePath = fileUri.toFilePath();
      if (!filePath.startsWith(rootPath)) {
        request.response.statusCode = HttpStatus.forbidden;
        await request.response.close();
        return;
      }

      final file = File(filePath);
      if (!await file.exists()) {
        request.response.statusCode = HttpStatus.notFound;
        await request.response.close();
        return;
      }

      request.response.headers.contentType = _contentTypeForPath(file.path);
      request.response.headers.set(HttpHeaders.cacheControlHeader, 'no-store');
      if (request.method == 'HEAD') {
        request.response.contentLength = await file.length();
        await request.response.close();
        return;
      }
      await request.response.addStream(file.openRead());
      await request.response.close();
    } catch (error) {
      request.response.statusCode = HttpStatus.internalServerError;
      await request.response.close();
    }
  }

  ContentType _contentTypeForPath(String path) {
    final lower = path.toLowerCase();
    if (lower.endsWith('.html')) {
      return ContentType.html;
    }
    if (lower.endsWith('.js')) {
      return ContentType('application', 'javascript', charset: 'utf-8');
    }
    if (lower.endsWith('.json')) {
      return ContentType.json;
    }
    if (lower.endsWith('.png')) {
      return ContentType('image', 'png');
    }
    if (lower.endsWith('.jpg') || lower.endsWith('.jpeg')) {
      return ContentType('image', 'jpeg');
    }
    if (lower.endsWith('.moc3')) {
      return ContentType('application', 'octet-stream');
    }
    return ContentType.binary;
  }

  Future<void> _hideDiagnostics() async {
    await _callJS('window.setDiagnosticsVisible(false)');
  }

  String _jsDouble(double value) => value.toStringAsFixed(4);

  Future<void> _syncModelViewTransform() async {
    await _callJS(
      'window.setUserViewTransform('
      '${_jsDouble(_modelUserScale)}, '
      '${_jsDouble(_modelUserOffsetX)}, '
      '${_jsDouble(_modelUserOffsetY)})',
    );
    await _refreshLive2dDebugState();
  }

  Future<void> _callJS(String js) async {
    if (!_webViewReady) return;
    final ctrl = _webCtrl;
    if (ctrl == null) {
      return;
    }
    await _evaluateJS(ctrl, js, logPrefix: 'Cortana JS Call');
  }

  Future<dynamic> _evaluateJS(
    InAppWebViewController ctrl,
    String js, {
    required String logPrefix,
  }) async {
    final viewId = ctrl.getViewId();
    if (viewId == null) {
      _appendLog('JS 调用跳过: WebView 尚未完成原生绑定');
      return null;
    }
    try {
      final result = await ctrl.evaluateJavascript(source: js);
      debugPrint('[$logPrefix] $js => $result');
      return result;
    } on MissingPluginException catch (error, stackTrace) {
      // Native method channel may not be registered yet — retry once after a
      // short delay instead of immediately failing.
      debugPrint(
        '[$logPrefix Retry] evaluateJavascript threw MissingPluginException, '
        'retrying in 200ms: $error',
      );
      await Future<void>.delayed(const Duration(milliseconds: 200));
      try {
        final result = await ctrl.evaluateJavascript(source: js);
        debugPrint('[$logPrefix Retry OK] $js => $result');
        return result;
      } catch (retryError, retryStack) {
        debugPrint('[$logPrefix Retry Failed] $js => $retryError');
        debugPrint('$retryStack');
        _appendLog('JS 调用失败: WebView 原生插件未就绪，请完全重启应用');
      }
    } catch (error, stackTrace) {
      debugPrint('[$logPrefix Error] $js => $error');
      debugPrint('$stackTrace');
      _appendLog('JS 调用失败: $error');
    }
    return null;
  }

  Future<void> _refreshLive2dDebugState() async {
    if (!_webViewReady) {
      return;
    }
    final ctrl = _webCtrl;
    if (ctrl == null) {
      return;
    }
    try {
      final state = await _evaluateJS(
        ctrl,
        'JSON.stringify(window.cortanaDebugState ? window.cortanaDebugState() : null);',
        logPrefix: 'Cortana JS Debug',
      );
      final raw = state?.toString().trim() ?? '';
      if (raw.isEmpty || raw == 'null' || raw == 'undefined') {
        return;
      }
      final decoded = jsonDecode(raw);
      if (decoded is! Map) {
        return;
      }
      final normalized = Map<String, dynamic>.from(decoded);
      if (!mounted) {
        _live2dDebugState = normalized;
        return;
      }
      setState(() {
        _live2dDebugState = normalized;
      });
    } catch (error) {
      _appendLog('刷新 Live2D 状态失败: $error');
    }
  }

  InAppWebView _buildWebView({
    String? initialFile,
    InAppWebViewInitialData? initialData,
    URLRequest? initialUrlRequest,
  }) {
    return InAppWebView(
      initialFile: initialFile,
      initialData: initialData,
      initialUrlRequest: initialUrlRequest,
      initialSettings: InAppWebViewSettings(
        transparentBackground: true,
        allowFileAccessFromFileURLs: true,
        allowUniversalAccessFromFileURLs: true,
        javaScriptEnabled: true,
        isInspectable: true,
      ),
      onWebViewCreated: (ctrl) {
        _webCtrl = ctrl;
        debugPrint('[Cortana] WebView created');
        ctrl.addJavaScriptHandler(
          handlerName: _jsLogHandlerName,
          callback: (args) {
            final message = args.isEmpty ? '' : args.first.toString();
            if (_shouldSurfaceCortanaWebLog(message)) {
              _appendLog('WebView JS: $message');
            }
            return {'ok': true};
          },
        );
      },
      onLoadStart: (ctrl, url) {
        _webViewReady = false;
        debugPrint('[Cortana] Load start: $url');
        _updateLoadStatus('WebView load start: $url');
      },
      onLoadStop: (ctrl, url) async {
        debugPrint('[Cortana] Load stop: $url');
        _updateLoadStatus('WebView load stop: $url');
        _webCtrl = ctrl;
        // Delay to allow the native plugin channel to finish registering
        // before marking the WebView as ready. Without this, evaluateJavascript
        // can throw MissingPluginException on Android.
        await Future<void>.delayed(const Duration(milliseconds: 300));
        _webViewReady = true;
        await _hideDiagnostics();
        await _syncModelViewTransform();
        await _refreshLive2dDebugState();
      },
      onConsoleMessage: (ctrl, msg) {
        final message = msg.message;
        if (_shouldSurfaceCortanaWebLog(message)) {
          _appendLog('WebView console: $message');
        }
      },
      onReceivedError: (ctrl, request, error) {
        debugPrint(
          '[Cortana Error] ${error.type}: ${error.description} (${request.url})',
        );
        _updateLoadStatus(
          'WebView error: ${error.description} (${request.url})',
        );
      },
      onReceivedHttpError: (ctrl, request, response) {
        debugPrint(
          '[Cortana HTTP Error] ${response.statusCode} ${response.reasonPhrase} (${request.url})',
        );
        _updateLoadStatus(
          'HTTP ${response.statusCode} ${response.reasonPhrase} (${request.url})',
        );
      },
    );
  }

  void _resetPlaybackEffects() {
    _lipTimer?.cancel();
    _lipTimer = null;
    unawaited(_audioPositionSub?.cancel());
    _audioPositionSub = null;
    for (final timer in _motionTimers) {
      timer.cancel();
    }
    _motionTimers.clear();
  }

  String _normalizeExpression(String raw) {
    final key = raw.trim();
    if (key.isEmpty) {
      return 'happy';
    }
    return _expressionAliases[key] ??
        _expressionAliases[key.toLowerCase()] ??
        key;
  }

  String _normalizeMotion(String raw) {
    final key = raw.trim();
    if (key.isEmpty) {
      return 'Idle';
    }
    return _motionAliases[key] ?? _motionAliases[key.toLowerCase()] ?? key;
  }

  Map<String, dynamic>? _normalizeRemoteActionPlan(
    Map<String, dynamic>? rawPlan,
  ) {
    if (rawPlan == null || rawPlan.isEmpty) {
      return null;
    }
    final normalized = <String, dynamic>{};
    final expression = _normalizeExpression(
      (rawPlan['expression'] ?? '').toString(),
    );
    if (expression.isNotEmpty) {
      normalized['expression'] = expression;
    }
    final fallbackExpression = _normalizeExpression(
      (rawPlan['fallback_expression'] ??
              rawPlan['expression_fallback'] ??
              'happy')
          .toString(),
    );
    normalized['fallback_expression'] = fallbackExpression;
    final expressionHoldMsRaw =
        rawPlan['expression_hold_ms'] ?? rawPlan['hold_expression_ms'];
    final expressionHoldMs = expressionHoldMsRaw is int
        ? expressionHoldMsRaw
        : int.tryParse('$expressionHoldMsRaw') ?? 0;
    if (expressionHoldMs > 0) {
      normalized['expression_hold_ms'] = expressionHoldMs;
    }
    final mood = (rawPlan['mood'] ?? '').toString().trim();
    if (mood.isNotEmpty) {
      normalized['mood'] = mood;
    }
    final rawActions = rawPlan['actions'];
    if (rawActions is List) {
      normalized['actions'] = rawActions.map((action) {
        final item = action is Map
            ? Map<String, dynamic>.from(action)
            : <String, dynamic>{};
        final rawDelay = item['delay'];
        final delay = rawDelay is int
            ? rawDelay
            : int.tryParse('$rawDelay') ?? 0;
        final rawIndex = item['index'];
        final index = rawIndex is int
            ? rawIndex
            : int.tryParse('$rawIndex') ?? 0;
        final rawHoldMs = item['hold_ms'];
        final holdMs = rawHoldMs is int
            ? rawHoldMs
            : int.tryParse('$rawHoldMs') ?? 0;
        final resumeToIdle =
            item['resume_to_idle'] == true ||
            item['resume_to_idle']?.toString().toLowerCase() == 'true';
        return <String, dynamic>{
          'motion': _normalizeMotion((item['motion'] ?? '').toString()),
          'delay': delay,
          'index': index,
          if (holdMs > 0) 'hold_ms': holdMs,
          if (resumeToIdle) 'resume_to_idle': true,
        };
      }).toList();
    }
    return normalized.isEmpty ? null : normalized;
  }

  Map<String, dynamic> _getActionPlan(
    String replyText, {
    required bool hasAudio,
    Map<String, dynamic>? remoteActionPlan,
  }) {
    final normalizedRemote = _normalizeRemoteActionPlan(remoteActionPlan);
    if (normalizedRemote != null) {
      return normalizedRemote;
    }
    final normalized = replyText.trim();
    final length = normalized.length;
    final isGreeting = normalized.contains(
      RegExp(r'你好|您好|嗨|hi|hello', caseSensitive: false),
    );
    final isApology = normalized.contains(RegExp(r'抱歉|对不起|遗憾|不好意思'));
    final hasEmphasis = normalized.contains(RegExp(r'！|!|哇|真的|竟然|请注意|重点'));
    final asksQuestion = normalized.contains(RegExp(r'？|\\?|吗|呢|如何|怎么'));

    String expression = 'happy';
    if (isApology) {
      expression = 'sad';
    } else if (hasEmphasis) {
      expression = 'surprised';
    }

    final actions = <Map<String, dynamic>>[];
    void pushAction(String motion, int delay) {
      actions.add(<String, dynamic>{
        'motion': _normalizeMotion(motion),
        'delay': delay,
      });
    }

    if (isGreeting) {
      pushAction('IdleWave', 0);
      pushAction('Idle', 2200);
    } else if (length < 24) {
      pushAction(asksQuestion ? 'IdleAlt' : 'Idle', 0);
      if (hasEmphasis) {
        pushAction('Tap', 1400);
      }
    } else if (length < 80) {
      pushAction('Idle', 0);
      pushAction(asksQuestion ? 'IdleAlt' : 'Tap', 1800);
      pushAction('Idle', 4200);
    } else {
      pushAction('Idle', 0);
      pushAction('Tap', 1800);
      pushAction('IdleAlt', 4200);
      pushAction('Idle', 7000);
    }

    if (hasAudio && actions.every((action) => action['motion'] != 'IdleAlt')) {
      pushAction('IdleAlt', 5600);
    }

    return <String, dynamic>{
      'expression': _normalizeExpression(expression),
      'fallback_expression': 'happy',
      'actions': actions,
    };
  }

  int _estimateSpeechDurationMs(String text) {
    final normalized = text.trim();
    if (normalized.isEmpty) {
      return 1800;
    }
    final runeCount = normalized.runes.length;
    final punctuationCount = RegExp(
      r'[，。！？；：,.!?;:]',
    ).allMatches(normalized).length;
    final estimated = 1200 + runeCount * 165 + punctuationCount * 220;
    return estimated.clamp(1800, 14000);
  }

  List<double> _buildLipSyncProfile(String text) {
    final normalized = text.trim();
    if (normalized.isEmpty) {
      return const <double>[0.26, 0.52, 0.34, 0.58, 0.22];
    }
    final chunks = <String>[];
    final buffer = StringBuffer();
    for (final rune in normalized.runes) {
      final char = String.fromCharCode(rune);
      buffer.write(char);
      final isBoundary = RegExp(r'[，。！？；：,.!?;:]').hasMatch(char);
      if (buffer.length >= 4 || isBoundary) {
        chunks.add(buffer.toString());
        buffer.clear();
      }
    }
    if (buffer.isNotEmpty) {
      chunks.add(buffer.toString());
    }
    return chunks.map((chunk) {
      final emphasis = RegExp(r'[！!？?]').hasMatch(chunk) ? 0.18 : 0.0;
      final softness = RegExp(r'[，。；：,.;:]').hasMatch(chunk) ? -0.1 : 0.0;
      final base = 0.36 + (chunk.runes.length.clamp(1, 6) * 0.07);
      return (base + emphasis + softness).clamp(0.16, 0.92);
    }).toList();
  }

  double _lipAmplitudeForProgress(
    double progress,
    List<double> profile,
    DateTime startedAt,
    double previousAmplitude,
  ) {
    if (profile.isEmpty) {
      return 0.24;
    }
    final clamped = progress.clamp(0.0, 1.0);
    final scaled = clamped * profile.length;
    final segmentIndex = scaled.floor().clamp(0, profile.length - 1);
    final localT = scaled - scaled.floor();
    final current = profile[segmentIndex];
    final next = profile[(segmentIndex + 1).clamp(0, profile.length - 1)];
    final blended = current + (next - current) * localT;
    final elapsedMs = DateTime.now().difference(startedAt).inMilliseconds;
    final pulse = 0.04 * (1 + math.sin(elapsedMs / 260.0)) * 0.5;
    final target = (blended + pulse).clamp(0.12, 0.9);
    return (previousAmplitude + (target - previousAmplitude) * 0.28).clamp(
      0.12,
      0.9,
    );
  }

  void _startLipSyncLoop(String replyText) {
    _lipTimer?.cancel();
    _audioPositionSub?.cancel();
    _audioPositionSub = null;
    final estimatedDurationMs = _estimateSpeechDurationMs(replyText);
    final profile = _buildLipSyncProfile(replyText);
    final startedAt = DateTime.now();
    var currentAmplitude = 0.22;

    void pushLipSync(Duration position) {
      final progress = position.inMilliseconds / estimatedDurationMs;
      currentAmplitude = _lipAmplitudeForProgress(
        progress,
        profile,
        startedAt,
        currentAmplitude,
      );
      _callJS('window.startLipSync(${_jsDouble(currentAmplitude)})');
    }

    _audioPositionSub = _audio.onPositionChanged.listen(pushLipSync);
    _lipTimer = Timer.periodic(const Duration(milliseconds: 180), (_) {
      final elapsed = DateTime.now().difference(startedAt);
      pushLipSync(elapsed);
    });
  }

  void _scheduleActions(List<dynamic> actions) {
    for (final action in actions) {
      final item = action is Map<String, dynamic>
          ? action
          : action is Map
          ? Map<String, dynamic>.from(action)
          : const <String, dynamic>{};
      final motion = _normalizeMotion((item['motion'] ?? 'Idle').toString());
      final rawDelay = item['delay'];
      final delay = rawDelay is int ? rawDelay : int.tryParse('$rawDelay') ?? 0;
      final rawIndex = item['index'];
      final index = rawIndex is int ? rawIndex : int.tryParse('$rawIndex') ?? 0;
      final rawHoldMs = item['hold_ms'];
      final holdMs = rawHoldMs is int
          ? rawHoldMs
          : int.tryParse('$rawHoldMs') ?? 0;
      final resumeToIdle =
          item['resume_to_idle'] == true ||
          item['resume_to_idle']?.toString().toLowerCase() == 'true';
      final timer = Timer(Duration(milliseconds: delay), () {
        _callJS("window.setMotion('$motion', $index)");
        if (resumeToIdle && holdMs > 0) {
          final settleTimer = Timer(Duration(milliseconds: holdMs), () {
            _callJS("window.setMotion('Idle', 0)");
          });
          _motionTimers.add(settleTimer);
        }
      });
      _motionTimers.add(timer);
    }
  }

  void _rememberVoiceHistory(
    CortanaReplyPayload reply, {
    required String audioPath,
    Uint8List? audioBytes,
    required String audioFormat,
  }) {
    final text = reply.text.trim();
    if ((audioPath.trim().isEmpty && audioBytes == null) || text.isEmpty) {
      return;
    }
    final item = _CortanaVoiceHistoryItem(
      id: '${DateTime.now().microsecondsSinceEpoch}_$audioPath',
      text: text,
      audioPath: audioPath,
      audioBytes: audioBytes,
      audioFormat: audioFormat,
      createdAt: DateTime.now(),
      actionPlan: reply.actionPlan == null
          ? null
          : Map<String, dynamic>.from(reply.actionPlan!),
    );
    if (!mounted) {
      _voiceHistory.insert(0, item);
      if (_voiceHistory.length > 3) {
        _voiceHistory.removeRange(3, _voiceHistory.length);
      }
      return;
    }
    setState(() {
      _voiceHistory.insert(0, item);
      if (_voiceHistory.length > 3) {
        _voiceHistory.removeRange(3, _voiceHistory.length);
      }
    });
  }

  List<CortanaReplayItem> _combinedVoiceHistory() {
    final combined = <CortanaReplayItem>[];
    final seenIds = <String>{};

    void addItem(CortanaReplayItem item) {
      if (!seenIds.add(item.id)) {
        return;
      }
      combined.add(item);
    }

    for (final item in widget.externalVoiceHistory) {
      addItem(item);
    }
    for (final item in _voiceHistory) {
      addItem(
        CortanaReplayItem(
          id: item.id,
          text: item.text,
          audioPath: item.audioPath,
          audioBytes: item.audioBytes,
          audioFormat: item.audioFormat,
          createdAt: item.createdAt,
          actionPlan: item.actionPlan,
          sourceLabel: 'Cortana',
        ),
      );
    }

    combined.sort((a, b) => b.createdAt.compareTo(a.createdAt));
    if (combined.length <= 6) {
      return combined;
    }
    return combined.sublist(0, 6);
  }

  Future<void> _playReplyAudio(
    CortanaReplyPayload reply, {
    bool showSnackBar = true,
    bool rememberHistory = true,
  }) async {
    final replyText = reply.text.trim();
    if (replyText.isEmpty) {
      throw Exception('LLM returned empty response');
    }

    final playbackToken = ++_playbackToken;
    final actionPlan = _getActionPlan(
      replyText,
      hasAudio: reply.hasAudio,
      remoteActionPlan: reply.actionPlan,
    );
    final expression = _normalizeExpression(
      (actionPlan['expression'] ?? 'happy').toString(),
    );
    final fallbackExpression = _normalizeExpression(
      (actionPlan['fallback_expression'] ?? 'happy').toString(),
    );
    final rawExpressionHoldMs = actionPlan['expression_hold_ms'];
    final expressionHoldMs = rawExpressionHoldMs is int
        ? rawExpressionHoldMs
        : int.tryParse('$rawExpressionHoldMs') ?? 0;

    _resetPlaybackEffects();
    await _audio.stop();
    await _callJS('window.stopLipSync()');
    if (playbackToken != _playbackToken) {
      return;
    }

    if (expressionHoldMs > 0) {
      await _callJS(
        "window.setExpressionFor('$expression', $expressionHoldMs, '$fallbackExpression')",
      );
    } else {
      await _callJS("window.setExpression('$expression')");
    }

    final actions = actionPlan['actions'] as List<dynamic>? ?? [];
    _scheduleActions(actions);

    String audioPath = reply.audioPath.trim();
    Uint8List? audioBytes = reply.audioBytes;
    String audioFormat = reply.audioFormat.trim();
    if (playbackToken != _playbackToken) {
      return;
    }

    if (audioPath.isEmpty && audioBytes == null) {
      _appendLog('LLM 未返回可播放语音，本次仅展示文本回复');
      _resetPlaybackEffects();
      await _callJS('window.stopLipSync()');
      if (showSnackBar && mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Cortana: $replyText'),
            duration: const Duration(seconds: 3),
          ),
        );
      }
      return;
    }

    var speechFocusStarted = false;
    try {
      await _callJS('window.beginSpeechFocus()');
      speechFocusStarted = true;
      _startLipSyncLoop(replyText);

      if (audioPath.isNotEmpty) {
        await _audio.play(DeviceFileSource(audioPath));
      } else if (audioBytes != null) {
        await _audio.play(BytesSource(audioBytes));
      } else {
        throw Exception('No playable audio source');
      }
      await _audio.onPlayerComplete.first;

      if (playbackToken != _playbackToken) {
        return;
      }

      if (rememberHistory) {
        _rememberVoiceHistory(
          CortanaReplyPayload(
            text: replyText,
            audioPath: audioPath,
            audioBytes: audioBytes,
            audioFormat: audioFormat,
            actionPlan: reply.actionPlan,
            requestId: reply.requestId,
          ),
          audioPath: audioPath,
          audioBytes: audioBytes,
          audioFormat: audioFormat,
        );
      }
    } finally {
      _resetPlaybackEffects();
      await _callJS('window.stopLipSync()');
      if (speechFocusStarted) {
        await _callJS('window.endSpeechFocus()');
      }
    }

    if (showSnackBar && mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text('Cortana: $replyText'),
          duration: const Duration(seconds: 3),
        ),
      );
    }
  }

  Future<void> _replayHistory(_CortanaVoiceHistoryItem item) async {
    if (_speaking) {
      return;
    }

    setState(() => _speaking = true);
    try {
      await _playReplyAudio(
        CortanaReplyPayload(
          text: item.text,
          audioPath: item.audioPath,
          audioBytes: item.audioBytes,
          audioFormat: item.audioFormat,
          actionPlan: item.actionPlan,
        ),
        rememberHistory: false,
      );
    } catch (e, stackTrace) {
      debugPrint('[Cortana Replay Error] $e');
      debugPrint('$stackTrace');
      _appendLog('历史重播失败: $e');
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('重播失败: $e'),
            duration: const Duration(seconds: 3),
            backgroundColor: Colors.red,
          ),
        );
      }
    } finally {
      if (mounted) {
        setState(() => _speaking = false);
      }
    }
  }

  Future<void> _replayLatestExternalVoice() async {
    if (_speaking) {
      return;
    }

    final latest = widget.externalVoiceHistory.isEmpty
        ? null
        : widget.externalVoiceHistory.first;
    if (latest == null) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('聊天页签里还没有可播放的语音'),
            duration: Duration(seconds: 2),
          ),
        );
      }
      return;
    }

    setState(() => _speaking = true);
    try {
      _appendLog('输入为空，重播聊天页签最近一条语音');
      await _playReplyAudio(
        CortanaReplyPayload(
          text: latest.text,
          audioPath: latest.audioPath,
          audioBytes: latest.audioBytes,
          audioFormat: latest.audioFormat,
          actionPlan: latest.actionPlan,
        ),
        rememberHistory: false,
      );
    } catch (e, stackTrace) {
      debugPrint('[Cortana Replay Latest External Error] $e');
      debugPrint('$stackTrace');
      _appendLog('重播聊天页签语音失败: $e');
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('重播失败: $e'),
            duration: const Duration(seconds: 3),
            backgroundColor: Colors.red,
          ),
        );
      }
    } finally {
      if (mounted) {
        setState(() => _speaking = false);
      }
    }
  }

  /// 播放服务器推送的播报（无需用户输入，自动播放）
  Future<void> playBroadcast(
    CortanaReplyPayload payload, {
    VoidCallback? onFinished,
  }) async {
    if (_speaking) {
      _queuedBroadcasts.addLast(_QueuedBroadcast(payload, onFinished));
      return;
    }

    return _playBroadcastNow(payload, onFinished: onFinished);
  }

  Future<void> _playBroadcastNow(
    CortanaReplyPayload payload, {
    VoidCallback? onFinished,
  }) async {
    var finishNotified = false;

    void notifyFinished() {
      if (finishNotified) {
        return;
      }
      finishNotified = true;
      onFinished?.call();
    }

    setState(() => _speaking = true);
    _broadcastAutoCollapseTimer?.cancel();

    try {
      debugPrint('[Cortana Broadcast] Playing: ${payload.text}');
      _appendLog('播报: ${payload.text}');
      await _playReplyAudio(payload);
      notifyFinished();
    } catch (e, stackTrace) {
      debugPrint('[Cortana Broadcast Error] $e');
      debugPrint('$stackTrace');
      _appendLog('播报失败: $e');
      _resetPlaybackEffects();
      await _callJS('window.stopLipSync()');
      await _callJS('window.endSpeechFocus()');
    } finally {
      notifyFinished();
      if (mounted) {
        setState(() => _speaking = false);
      }
      if (_queuedBroadcasts.isNotEmpty) {
        final next = _queuedBroadcasts.removeFirst();
        unawaited(_playBroadcastNow(next.payload, onFinished: next.onFinished));
      } else if (mounted) {
        _startAutoCollapseTimer();
      }
    }
  }

  void _startAutoCollapseTimer() {
    _broadcastAutoCollapseTimer?.cancel();
    _broadcastAutoCollapseTimer = Timer(widget.autoCollapseDelay, () {
      if (!mounted) return;
      if (widget.mode != CortanaDisplayMode.fullscreen &&
          widget.mode != CortanaDisplayMode.collapsed) {
        widget.onModeChanged?.call(CortanaDisplayMode.collapsed);
      }
    });
  }

  Future<void> _speak(String text) async {
    if (_speaking) {
      return;
    }
    if (text.isEmpty) {
      await _replayLatestExternalVoice();
      return;
    }

    // 检查是否有消息发送回调
    if (widget.onSendMessage == null) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('请先登录'),
            duration: Duration(seconds: 2),
            backgroundColor: Colors.orange,
          ),
        );
      }
      return;
    }

    setState(() => _speaking = true);

    try {
      debugPrint('[Cortana] Sending message: $text');
      _appendLog('发送请求: $text');
      final reply = await widget.onSendMessage!(text);
      debugPrint(
        '[Cortana LLM] User: $text, Reply: ${reply.text.trim()}, audio=${reply.audioPath}, request=${reply.requestId}',
      );
      await _playReplyAudio(reply);
    } catch (e, stackTrace) {
      debugPrint('[Cortana Error] $e');
      debugPrint('$stackTrace');
      _appendLog('对话失败: $e');
      _resetPlaybackEffects();
      await _callJS('window.stopLipSync()');
      await _callJS('window.endSpeechFocus()');

      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('对话失败: $e'),
            duration: const Duration(seconds: 3),
            backgroundColor: Colors.red,
          ),
        );
      }
    } finally {
      if (mounted) {
        setState(() => _speaking = false);
      }
    }
  }

  Future<void> speakText(String text) => _speak(text.trim());

  Size _floatingSizeForMode(CortanaDisplayMode mode) {
    switch (mode) {
      case CortanaDisplayMode.collapsed:
        return const Size(_floatingCollapsedSize, _floatingCollapsedSize);
      case CortanaDisplayMode.small:
        return const Size(_floatingSmallWidth, _floatingSmallHeight);
      case CortanaDisplayMode.expanded:
        return const Size(_floatingExpandedWidth, _floatingExpandedHeight);
      case CortanaDisplayMode.fullscreen:
        final size = MediaQuery.sizeOf(context);
        return size;
    }
  }

  CortanaDisplayMode _nextFloatingMode(CortanaDisplayMode current) {
    switch (current) {
      case CortanaDisplayMode.collapsed:
        return CortanaDisplayMode.small;
      case CortanaDisplayMode.small:
        return CortanaDisplayMode.expanded;
      case CortanaDisplayMode.expanded:
        return CortanaDisplayMode.collapsed;
      case CortanaDisplayMode.fullscreen:
        return CortanaDisplayMode.expanded;
    }
  }

  Offset _defaultFloatingPosition(Size floatingSize) {
    final screenSize = MediaQuery.sizeOf(context);
    final bottomPadding = MediaQuery.paddingOf(context).bottom;
    final navBarHeight = 80.0;
    return Offset(
      screenSize.width - floatingSize.width - 12,
      screenSize.height -
          floatingSize.height -
          bottomPadding -
          widget.floatingBottomInset -
          navBarHeight -
          8,
    );
  }

  Offset _clampFloatingOffset(Offset offset, Size floatingSize) {
    final screenSize = MediaQuery.sizeOf(context);
    final topPadding = MediaQuery.paddingOf(context).top + kToolbarHeight;
    final bottomPadding =
        MediaQuery.paddingOf(context).bottom + 80 + widget.floatingBottomInset;
    return Offset(
      offset.dx.clamp(0.0, screenSize.width - floatingSize.width),
      offset.dy.clamp(
        topPadding,
        screenSize.height - floatingSize.height - bottomPadding,
      ),
    );
  }

  String _formatFlag(bool value) => value ? '已就绪' : '未就绪';

  String _shortValue(Object? value) {
    final text = (value ?? '').toString().trim();
    if (text.isEmpty) {
      return '-';
    }
    if (text.length <= 42) {
      return text;
    }
    return '${text.substring(0, 39)}...';
  }

  Widget _buildLive2dMetric(
    BuildContext context, {
    required String label,
    required String value,
    IconData? icon,
  }) {
    final cs = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
      decoration: BoxDecoration(
        color: cs.surfaceContainerHighest,
        borderRadius: BorderRadius.circular(12),
      ),
      child: Row(
        children: [
          if (icon != null) ...[
            Icon(icon, size: 16, color: cs.primary),
            const SizedBox(width: 8),
          ],
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(label, style: Theme.of(context).textTheme.labelMedium),
                const SizedBox(height: 2),
                Text(
                  value,
                  style: Theme.of(context).textTheme.bodyMedium,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildSectionCard({
    required Widget child,
    EdgeInsetsGeometry padding = const EdgeInsets.fromLTRB(12, 10, 12, 10),
  }) {
    final cs = Theme.of(context).colorScheme;
    return Container(
      decoration: BoxDecoration(
        color: cs.surfaceContainerLow,
        borderRadius: BorderRadius.circular(14),
      ),
      child: Padding(padding: padding, child: child),
    );
  }

  Widget _buildLive2dSummaryContent(BuildContext context) {
    final state = _live2dDebugState ?? const <String, dynamic>{};
    final userViewState = state['userViewState'] is Map
        ? Map<String, dynamic>.from(state['userViewState'] as Map)
        : const <String, dynamic>{};
    final speechFocusState = state['speechFocusState'] is Map
        ? Map<String, dynamic>.from(state['speechFocusState'] as Map)
        : const <String, dynamic>{};
    final lipSyncState = state['lipSyncState'] is Map
        ? Map<String, dynamic>.from(state['lipSyncState'] as Map)
        : const <String, dynamic>{};
    final lastModelConfig = state['lastModelConfig'] is Map
        ? Map<String, dynamic>.from(state['lastModelConfig'] as Map)
        : const <String, dynamic>{};

    final modelCreated = state['modelCreated'] == true;
    final appCreated = state['appCreated'] == true;
    final live2dPresent = state['live2dPresent'] == true;
    final speechActive = speechFocusState['active'] == true;
    final motionName = _shortValue(
      lastModelConfig['motion'] ?? lastModelConfig['motionGroup'],
    );
    final expressionName = _shortValue(lastModelConfig['expression']);
    final modelUrl = _shortValue(state['modelUrl']);
    final pixiVersion = _shortValue(state['pixiVersion']);
    final elapsedMs = (state['elapsedMs'] ?? 0).toString();
    final transformText =
        '缩放 ${((userViewState['scale'] ?? _modelUserScale) as num).toStringAsFixed(2)}'
        ' / X ${((userViewState['offsetX'] ?? _modelUserOffsetX) as num).toStringAsFixed(2)}'
        ' / Y ${((userViewState['offsetY'] ?? _modelUserOffsetY) as num).toStringAsFixed(2)}';
    final speechText =
        '${speechActive ? '播放中' : '空闲'}'
        ' · ${((speechFocusState['progress'] ?? 0) as num).toStringAsFixed(2)}';
    final lipText =
        '当前 ${((lipSyncState['current'] ?? 0) as num).toStringAsFixed(2)}'
        ' / 目标 ${((lipSyncState['target'] ?? 0) as num).toStringAsFixed(2)}';

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Align(
          alignment: Alignment.centerRight,
          child: IconButton(
            tooltip: '刷新数据',
            onPressed: () => unawaited(_refreshLive2dDebugState()),
            icon: const Icon(Icons.refresh, size: 18),
            visualDensity: VisualDensity.compact,
          ),
        ),
        const SizedBox(height: 4),
        Wrap(
          spacing: 8,
          runSpacing: 8,
          children: [
            _buildLive2dMetric(
              context,
              label: '渲染环境',
              value:
                  '${_formatFlag(appCreated)} / ${_formatFlag(live2dPresent)}',
              icon: Icons.layers,
            ),
            _buildLive2dMetric(
              context,
              label: '模型实例',
              value: _formatFlag(modelCreated),
              icon: Icons.face_retouching_natural,
            ),
            _buildLive2dMetric(
              context,
              label: 'Pixi',
              value: pixiVersion,
              icon: Icons.memory,
            ),
            _buildLive2dMetric(
              context,
              label: '初始化耗时',
              value: '$elapsedMs ms',
              icon: Icons.timelapse,
            ),
            _buildLive2dMetric(
              context,
              label: '当前表情',
              value: expressionName,
              icon: Icons.emoji_emotions_outlined,
            ),
            _buildLive2dMetric(
              context,
              label: '当前动作',
              value: motionName,
              icon: Icons.directions_run,
            ),
            _buildLive2dMetric(
              context,
              label: '语音状态',
              value: speechText,
              icon: Icons.graphic_eq,
            ),
            _buildLive2dMetric(
              context,
              label: '口型同步',
              value: lipText,
              icon: Icons.record_voice_over,
            ),
            _buildLive2dMetric(
              context,
              label: '视角参数',
              value: transformText,
              icon: Icons.threed_rotation,
            ),
            _buildLive2dMetric(
              context,
              label: '模型地址',
              value: modelUrl,
              icon: Icons.link,
            ),
          ],
        ),
      ],
    );
  }

  Widget _buildExpandableSectionCard({
    required String storageKey,
    required String title,
    required String subtitle,
    required bool expanded,
    required ValueChanged<bool> onExpansionChanged,
    required Widget child,
    Widget? trailing,
  }) {
    return _buildSectionCard(
      padding: EdgeInsets.zero,
      child: Theme(
        data: Theme.of(context).copyWith(dividerColor: Colors.transparent),
        child: ExpansionTile(
          key: PageStorageKey<String>(storageKey),
          initiallyExpanded: expanded,
          onExpansionChanged: onExpansionChanged,
          tilePadding: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
          title: Text(title),
          subtitle: Text(subtitle),
          trailing:
              trailing ??
              Icon(expanded ? Icons.expand_less : Icons.expand_more),
          children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(12, 0, 12, 10),
              child: child,
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildControlActionChip(
    BuildContext context, {
    required String label,
    required VoidCallback onPressed,
    IconData? icon,
  }) {
    final cs = Theme.of(context).colorScheme;
    return ActionChip(
      backgroundColor: cs.surface,
      side: BorderSide(color: cs.outlineVariant.withValues(alpha: 0.7)),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      avatar: icon == null ? null : Icon(icon, size: 14, color: cs.primary),
      label: Text(
        label,
        style: Theme.of(context).textTheme.labelLarge?.copyWith(
          color: cs.onSurface,
          fontWeight: FontWeight.w600,
        ),
      ),
      onPressed: onPressed,
    );
  }

  Widget _buildViewControlsContent() {
    final scaleText = _modelUserScale.toStringAsFixed(2);
    final offsetXText = _modelUserOffsetX.toStringAsFixed(2);
    final offsetYText = _modelUserOffsetY.toStringAsFixed(2);
    return Column(
      children: [
        Row(
          children: [
            const SizedBox(width: 52, child: Text('缩放')),
            Expanded(
              child: Slider(
                value: _modelUserScale,
                min: 0.8,
                max: 1.35,
                divisions: 22,
                label: scaleText,
                onChanged: (value) {
                  setState(() {
                    _modelUserScale = value;
                  });
                  unawaited(_syncModelViewTransform());
                },
              ),
            ),
            SizedBox(
              width: 42,
              child: Text(scaleText, textAlign: TextAlign.end),
            ),
          ],
        ),
        Row(
          children: [
            const SizedBox(width: 52, child: Text('左右')),
            Expanded(
              child: Slider(
                value: _modelUserOffsetX,
                min: -0.35,
                max: 0.35,
                divisions: 28,
                label: offsetXText,
                onChanged: (value) {
                  setState(() {
                    _modelUserOffsetX = value;
                  });
                  unawaited(_syncModelViewTransform());
                },
              ),
            ),
            SizedBox(
              width: 42,
              child: Text(offsetXText, textAlign: TextAlign.end),
            ),
          ],
        ),
        Row(
          children: [
            const SizedBox(width: 52, child: Text('上下')),
            Expanded(
              child: Slider(
                value: _modelUserOffsetY,
                min: -0.28,
                max: 0.28,
                divisions: 28,
                label: offsetYText,
                onChanged: (value) {
                  setState(() {
                    _modelUserOffsetY = value;
                  });
                  unawaited(_syncModelViewTransform());
                },
              ),
            ),
            SizedBox(
              width: 42,
              child: Text(offsetYText, textAlign: TextAlign.end),
            ),
          ],
        ),
      ],
    );
  }

  Widget _buildReplayHistoryContent(
    BuildContext context,
    List<CortanaReplayItem> replayHistory,
  ) {
    final cs = Theme.of(context).colorScheme;
    if (replayHistory.isEmpty) {
      return Text('暂无可重播语音', style: Theme.of(context).textTheme.bodyMedium);
    }
    return Column(
      children: [
        for (final item in replayHistory)
          Padding(
            padding: const EdgeInsets.only(bottom: 6),
            child: Material(
              color: cs.surfaceContainerHighest,
              borderRadius: BorderRadius.circular(12),
              child: ListTile(
                dense: true,
                contentPadding: const EdgeInsets.symmetric(
                  horizontal: 12,
                  vertical: 2,
                ),
                leading: const Icon(Icons.history),
                title: Text(
                  item.text,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
                subtitle: Text(
                  '${item.createdAt.hour.toString().padLeft(2, '0')}:${item.createdAt.minute.toString().padLeft(2, '0')}:${item.createdAt.second.toString().padLeft(2, '0')}'
                  '${item.audioFormat.trim().isEmpty ? '' : ' · ${item.audioFormat}'}'
                  '${item.sourceLabel.trim().isEmpty ? '' : ' · ${item.sourceLabel}'}',
                ),
                trailing: IconButton(
                  tooltip: '重播',
                  onPressed: _speaking
                      ? null
                      : () => _replayHistory(
                          _CortanaVoiceHistoryItem(
                            id: item.id,
                            text: item.text,
                            audioPath: item.audioPath,
                            audioBytes: item.audioBytes,
                            audioFormat: item.audioFormat,
                            createdAt: item.createdAt,
                            actionPlan: item.actionPlan,
                          ),
                        ),
                  icon: const Icon(Icons.play_arrow),
                ),
              ),
            ),
          ),
      ],
    );
  }

  Widget _buildControlPanel(
    BuildContext context, {
    required double overlayWidth,
    required List<CortanaReplayItem> replayHistory,
    required String scaleText,
    required String offsetXText,
    required String offsetYText,
  }) {
    return Positioned(
      left: 12,
      top: 58,
      child: SizedBox(
        width: overlayWidth,
        child: ConstrainedBox(
          constraints: BoxConstraints(
            maxHeight: MediaQuery.sizeOf(context).height * 0.68,
          ),
          child: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                _buildExpandableSectionCard(
                  storageKey: 'cortana-live2d-summary',
                  title: 'Live2D 数据',
                  subtitle: _live2dSummaryExpanded ? '点击收起' : '默认折叠，点击查看',
                  expanded: _live2dSummaryExpanded,
                  onExpansionChanged: (expanded) {
                    setState(() {
                      _live2dSummaryExpanded = expanded;
                    });
                  },
                  child: _buildLive2dSummaryContent(context),
                ),
                const SizedBox(height: 8),
                _buildExpandableSectionCard(
                  storageKey: 'cortana-expression-actions',
                  title: '表情与动作',
                  subtitle: _expressionActionsExpanded ? '点击收起' : '默认折叠，点击展开控制',
                  expanded: _expressionActionsExpanded,
                  onExpansionChanged: (expanded) {
                    setState(() {
                      _expressionActionsExpanded = expanded;
                    });
                  },
                  child: Wrap(
                    spacing: 6,
                    runSpacing: 6,
                    children: [
                      for (final e in _expressions)
                        _buildControlActionChip(
                          context,
                          label: e,
                          onPressed: () =>
                              _callJS("window.setExpression('$e')"),
                        ),
                      for (final m in _motions)
                        _buildControlActionChip(
                          context,
                          label: m,
                          icon: Icons.directions_run,
                          onPressed: () => _callJS(
                            "window.setMotion('${_normalizeMotion(m)}', 0)",
                          ),
                        ),
                    ],
                  ),
                ),
                const SizedBox(height: 8),
                _buildExpandableSectionCard(
                  storageKey: 'cortana-view-controls',
                  title: '视角调整',
                  subtitle:
                      '当前: 缩放 $scaleText / X $offsetXText / Y $offsetYText',
                  expanded: _viewControlsExpanded,
                  onExpansionChanged: (expanded) {
                    setState(() {
                      _viewControlsExpanded = expanded;
                    });
                  },
                  trailing: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      TextButton(
                        onPressed: () {
                          setState(() {
                            _modelUserScale = 1.0;
                            _modelUserOffsetX = 0.0;
                            _modelUserOffsetY = 0.0;
                          });
                          unawaited(_syncModelViewTransform());
                        },
                        child: const Text('重置'),
                      ),
                      Icon(
                        _viewControlsExpanded
                            ? Icons.expand_less
                            : Icons.expand_more,
                      ),
                    ],
                  ),
                  child: _buildViewControlsContent(),
                ),
                const SizedBox(height: 8),
                _buildExpandableSectionCard(
                  storageKey: 'cortana-replay-history',
                  title: '语音重播',
                  subtitle: replayHistory.isEmpty
                      ? '暂无记录，默认折叠'
                      : '共 ${replayHistory.length} 条，默认折叠',
                  expanded: _replayExpanded,
                  onExpansionChanged: (expanded) {
                    setState(() {
                      _replayExpanded = expanded;
                    });
                  },
                  child: _buildReplayHistoryContent(context, replayHistory),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildWebViewForPlatform() {
    if (kIsWeb) {
      return _buildWebCortanaBackdrop(context);
    }
    if (_usesCustomLive2dModel) {
      _customWebRootFuture ??= _prepareCustomWebRoot();
      return FutureBuilder<_CustomCortanaWebRoot>(
        key: ValueKey<int>(_webViewRevision),
        future: _customWebRootFuture,
        builder: (context, snapshot) {
          if (snapshot.hasError) {
            return Center(
              child: Text('Cortana custom model failed: ${snapshot.error}'),
            );
          }
          if (!snapshot.hasData) {
            return const Center(child: CircularProgressIndicator());
          }
          final webRoot = snapshot.data!;
          final root = webRoot.root;
          final htmlFile = File(
            '${root.path}${Platform.pathSeparator}index.html',
          );
          debugPrint(
            '[Cortana Live2D] loading custom runtime: ${htmlFile.uri} model=${webRoot.modelUrl}',
          );
          final url =
              !kIsWeb && defaultTargetPlatform == TargetPlatform.android
              ? 'http://localhost:$_localhostPort/$_cortanaLocalPath'
              : Uri.file(htmlFile.path).toString();
          return _buildWebView(
            initialUrlRequest: URLRequest(
              url: WebUri(url),
            ),
          );
        },
      );
    }
    if (!kIsWeb && defaultTargetPlatform == TargetPlatform.android) {
      return FutureBuilder<void>(
        key: ValueKey<int>(_webViewRevision),
        future: _androidLocalhostFuture,
        builder: (context, snapshot) {
          if (snapshot.hasError) {
            return Center(
              child: Text('Cortana localhost failed: ${snapshot.error}'),
            );
          }
          if (snapshot.connectionState != ConnectionState.done) {
            return const Center(child: CircularProgressIndicator());
          }
          return _buildWebView(
            initialUrlRequest: URLRequest(
              url: WebUri(
                'http://localhost:$_localhostPort/$_cortanaLocalPath',
              ),
            ),
          );
        },
      );
    }
    return KeyedSubtree(
      key: ValueKey<int>(_webViewRevision),
      child: _buildWebView(initialFile: _cortanaHtmlAsset),
    );
  }

  Widget _buildWebCortanaBackdrop(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    return DecoratedBox(
      decoration: BoxDecoration(
        color: Colors.transparent,
      ),
      child: Center(
        child: Icon(
          Icons.face_rounded,
          size: 96,
          color: cs.primary.withValues(alpha: 0.42),
        ),
      ),
    );
  }

  Widget _buildCollapsedAvatar(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    return Stack(
      children: [
        Container(
          decoration: BoxDecoration(
            shape: BoxShape.circle,
            gradient: LinearGradient(
              begin: Alignment.topLeft,
              end: Alignment.bottomRight,
              colors: [cs.primaryContainer, cs.tertiaryContainer],
            ),
          ),
          child: Center(
            child: Icon(
              Icons.face_rounded,
              size: 28,
              color: cs.onPrimaryContainer,
            ),
          ),
        ),
        if (widget.showBadge)
          Positioned(
            right: 2,
            top: 2,
            child: Container(
              width: 12,
              height: 12,
              decoration: BoxDecoration(
                shape: BoxShape.circle,
                color: cs.error,
                border: Border.all(color: cs.surface, width: 2),
              ),
            ),
          ),
      ],
    );
  }

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    final isFullscreen = widget.mode == CortanaDisplayMode.fullscreen;
    final isCollapsed = widget.mode == CortanaDisplayMode.collapsed;
    final isFloating = !isFullscreen;
    final floatingSize = isFullscreen
        ? null
        : _floatingSizeForMode(widget.mode);
    final borderRadius = isFullscreen
        ? BorderRadius.zero
        : BorderRadius.circular(isCollapsed ? floatingSize!.width / 2 : 16);

    // Calculate floating position
    Offset clampedOffset = Offset.zero;
    if (!isFullscreen) {
      final defaultPos = _defaultFloatingPosition(floatingSize!);
      final offset = _floatingOffset ?? defaultPos;
      clampedOffset = _clampFloatingOffset(offset, floatingSize);
    }

    final screenWidth = MediaQuery.sizeOf(context).width;
    final overlayWidth = math.min(
      screenWidth < 640 ? screenWidth - 72 : screenWidth * 0.28,
      300.0,
    );
    final scaleText = _modelUserScale.toStringAsFixed(2);
    final offsetXText = _modelUserOffsetX.toStringAsFixed(2);
    final offsetYText = _modelUserOffsetY.toStringAsFixed(2);
    final replayHistory = _combinedVoiceHistory();

    return Stack(
      children: [
        // WebView - always in AnimatedPositioned, transitions smoothly
        AnimatedPositioned(
          duration: _isDragging
              ? Duration.zero
              : const Duration(milliseconds: 350),
          curve: Curves.easeInOut,
          left: isFullscreen ? 0.0 : clampedOffset.dx,
          top: isFullscreen ? 0.0 : clampedOffset.dy,
          right: isFullscreen ? 0.0 : null,
          bottom: isFullscreen ? 0.0 : null,
          width: isFullscreen ? null : floatingSize?.width,
          height: isFullscreen ? null : floatingSize?.height,
          child: GestureDetector(
            behavior: HitTestBehavior.translucent,
            onTap: isFullscreen
                ? null
                : () {
                    if (widget.mode == CortanaDisplayMode.collapsed) {
                      widget.onTapWhenFloating?.call();
                      return;
                    }
                    widget.onModeChanged?.call(_nextFloatingMode(widget.mode));
                  },
            onLongPress: isFullscreen
                ? null
                : () {
                    widget.onLongPressWhenFloating?.call();
                  },
            onPanStart: isFullscreen
                ? null
                : (_) {
                    _isDragging = true;
                  },
            onPanUpdate: isFullscreen
                ? null
                : (details) {
                    if (!mounted) return;
                    setState(() {
                      _floatingOffset = _clampFloatingOffset(
                        (_floatingOffset ?? clampedOffset) + details.delta,
                        floatingSize!,
                      );
                    });
                  },
            onPanEnd: isFullscreen
                ? null
                : (_) {
                    if (!mounted) return;
                    _isDragging = false;
                    setState(() {
                      // Snap to nearest horizontal edge, keep vertical position
                      final size = _floatingSizeForMode(widget.mode);
                      final screenWidth = MediaQuery.sizeOf(context).width;
                      final currentOffset =
                          _floatingOffset ?? _defaultFloatingPosition(size);
                      final centerX = currentOffset.dx + size.width / 2;
                      final snapLeft = centerX < screenWidth / 2;
                      _floatingOffset = Offset(
                        snapLeft ? 12.0 : screenWidth - size.width - 12.0,
                        currentOffset.dy,
                      );
                    });
                  },
            child: Material(
              color: Colors.transparent,
              child: AnimatedContainer(
                duration: const Duration(milliseconds: 350),
                curve: Curves.easeInOut,
                decoration: BoxDecoration(
                  borderRadius: borderRadius,
                  border: isFloating && isCollapsed
                      ? Border.all(
                          color: cs.outlineVariant.withValues(alpha: 0.6),
                          width: 1.5,
                        )
                      : null,
                  boxShadow: isFloating && isCollapsed
                      ? [
                          BoxShadow(
                            color: cs.shadow.withValues(alpha: 0.25),
                            blurRadius: 12,
                            offset: const Offset(0, 4),
                          ),
                        ]
                      : null,
                ),
                clipBehavior: isFloating && !isCollapsed
                    ? Clip.none
                    : Clip.antiAlias,
                child: Stack(
                  fit: StackFit.expand,
                  children: [
                    // 浮动小窗只展示 Live2D，避免 Android WebView 在播报时抢占输入焦点。
                    if (!isCollapsed)
                      IgnorePointer(
                        ignoring: isFloating,
                        child: _buildWebViewForPlatform(),
                      )
                    else
                      _buildCollapsedAvatar(context),
                    // Drag handle indicator (floating only, non-collapsed)
                    if (!isFullscreen && !isCollapsed)
                      Positioned(
                        top: 0,
                        left: 0,
                        right: 0,
                        child: SizedBox(
                          height: 20,
                          child: Center(
                            child: Container(
                              width: 24,
                              height: 3,
                              margin: const EdgeInsets.only(top: 6),
                              decoration: BoxDecoration(
                                color: cs.onSurface.withValues(alpha: 0.22),
                                borderRadius: BorderRadius.circular(2),
                              ),
                            ),
                          ),
                        ),
                      ),
                    // Close/collapse button (floating only, non-collapsed)
                    if (!isFullscreen && !isCollapsed)
                      Positioned(
                        top: 4,
                        right: 4,
                        child: Material(
                          color: Colors.transparent,
                          child: InkWell(
                            borderRadius: BorderRadius.circular(12),
                            onTap: () {
                              widget.onModeChanged?.call(
                                CortanaDisplayMode.collapsed,
                              );
                            },
                            child: Container(
                              width: 24,
                              height: 24,
                              decoration: BoxDecoration(
                                color: cs.surface.withValues(alpha: 0.36),
                                shape: BoxShape.circle,
                              ),
                              child: Icon(
                                Icons.close,
                                size: 14,
                                color: cs.onSurface.withValues(alpha: 0.7),
                              ),
                            ),
                          ),
                        ),
                      ),
                  ],
                ),
              ),
            ),
          ),
        ),
        // Fullscreen overlays (only when in fullscreen mode)
        if (isFullscreen)
          SafeArea(
            child: Stack(
              children: [
                Positioned(
                  left: 12,
                  top: 12,
                  child: _buildSectionCard(
                    padding: const EdgeInsets.symmetric(
                      horizontal: 8,
                      vertical: 6,
                    ),
                    child: FilledButton.icon(
                      onPressed: () {
                        setState(() {
                          _controlPanelVisible = !_controlPanelVisible;
                        });
                      },
                      icon: Icon(
                        _controlPanelVisible
                            ? Icons.visibility_off_outlined
                            : Icons.tune,
                      ),
                      label: Text(_controlPanelVisible ? '隐藏面板' : '控制面板'),
                    ),
                  ),
                ),
                if (_controlPanelVisible)
                  _buildControlPanel(
                    context,
                    overlayWidth: overlayWidth,
                    replayHistory: replayHistory,
                    scaleText: scaleText,
                    offsetXText: offsetXText,
                    offsetYText: offsetYText,
                  ),
              ],
            ),
          ),
      ],
    );
  }
}
