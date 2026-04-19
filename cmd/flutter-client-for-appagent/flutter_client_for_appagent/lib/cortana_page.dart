import 'dart:async';
import 'dart:convert';
import 'dart:math' as math;
import 'package:audioplayers/audioplayers.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_inappwebview/flutter_inappwebview.dart';

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
  });

  final String id;
  final String text;
  final String audioPath;
  final Uint8List? audioBytes;
  final String audioFormat;
  final DateTime createdAt;
  final Map<String, dynamic>? actionPlan;
  final String sourceLabel;
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

class CortanaPage extends StatefulWidget {
  const CortanaPage({
    super.key,
    this.onSendMessage,
    this.externalVoiceHistory = const <CortanaReplayItem>[],
  });

  final Future<CortanaReplyPayload> Function(String message)? onSendMessage;
  final List<CortanaReplayItem> externalVoiceHistory;

  @override
  State<CortanaPage> createState() => _CortanaPageState();
}

class _CortanaPageState extends State<CortanaPage> {
  static const _jsLogHandlerName = 'cortanaLog';
  static const _cortanaHtmlAsset = 'assets/cortana/index.html';
  static const _cortanaLocalPath = 'index.html';
  static const _localhostPort = 18080;
  InAppWebViewController? _webCtrl;
  final TextEditingController _textCtrl = TextEditingController();
  final AudioPlayer _audio = AudioPlayer();
  Timer? _lipTimer;
  Timer? _debugStateTimer;
  StreamSubscription<Duration>? _audioPositionSub;
  final List<Timer> _motionTimers = <Timer>[];
  bool _speaking = false;
  InAppLocalhostServer? _localhostServer;
  Future<void>? _androidLocalhostFuture;
  String? _androidLoadStatus;
  bool _showLogs = false;
  int _playbackToken = 0;
  final List<String> _runtimeLogs = <String>[];
  final List<_CortanaVoiceHistoryItem> _voiceHistory = <_CortanaVoiceHistoryItem>[];
  double _modelUserScale = 1.0;
  double _modelUserOffsetX = 0.0;
  double _modelUserOffsetY = 0.0;
  bool _live2dSummaryExpanded = false;
  bool _expressionActionsExpanded = false;
  bool _viewControlsExpanded = false;
  bool _replayExpanded = false;
  Map<String, dynamic>? _live2dDebugState;

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
      _localhostServer = InAppLocalhostServer(
        documentRoot: 'assets/cortana',
        port: _localhostPort,
      );
      _androidLoadStatus = 'Starting localhost server on $_localhostPort';
      _appendLog(_androidLoadStatus!);
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
    _debugStateTimer = Timer.periodic(
      const Duration(seconds: 3),
      (_) => unawaited(_refreshLive2dDebugState()),
    );
  }

  @override
  void dispose() {
    _resetPlaybackEffects();
    _debugStateTimer?.cancel();
    _audio.dispose();
    _textCtrl.dispose();
    final localhostServer = _localhostServer;
    if (localhostServer != null) {
      unawaited(localhostServer.close());
    }
    super.dispose();
  }

  void _appendLog(String message) {
    final text = message.trim();
    if (text.isEmpty) {
      return;
    }
    final now = DateTime.now();
    final hh = now.hour.toString().padLeft(2, '0');
    final mm = now.minute.toString().padLeft(2, '0');
    final ss = now.second.toString().padLeft(2, '0');
    final entry = '[$hh:$mm:$ss] $text';
    debugPrint('[Cortana Log] $entry');
    if (!mounted) {
      _runtimeLogs.insert(0, entry);
      if (_runtimeLogs.length > 40) {
        _runtimeLogs.removeRange(40, _runtimeLogs.length);
      }
      return;
    }
    setState(() {
      _runtimeLogs.insert(0, entry);
      if (_runtimeLogs.length > 40) {
        _runtimeLogs.removeRange(40, _runtimeLogs.length);
      }
    });
  }

  void _updateLoadStatus(String message) {
    if (!mounted) {
      _androidLoadStatus = message;
      return;
    }
    setState(() {
      _androidLoadStatus = message;
    });
    _appendLog(message);
  }

  Future<void> _syncDiagnosticsVisibility() async {
    await _callJS('window.setDiagnosticsVisible(${_showLogs ? 'true' : 'false'})');
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
    try {
      final result = await _webCtrl?.evaluateJavascript(source: js);
      debugPrint('[Cortana JS Call] $js => $result');
    } catch (error, stackTrace) {
      debugPrint('[Cortana JS Call Error] $js => $error');
      debugPrint('$stackTrace');
      _appendLog('JS 调用失败: $error');
    }
  }

  Future<void> _refreshLive2dDebugState() async {
    final ctrl = _webCtrl;
    if (ctrl == null) {
      return;
    }
    try {
      final state = await ctrl.evaluateJavascript(
        source: 'JSON.stringify(window.cortanaDebugState ? window.cortanaDebugState() : null);',
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
            final payload = args.isNotEmpty ? args.first : null;
            debugPrint('[Cortana Bridge] $payload');
            _appendLog('Bridge: $payload');
            return {'ok': true};
          },
        );
      },
      onLoadStart: (ctrl, url) {
        debugPrint('[Cortana] Load start: $url');
        _updateLoadStatus('WebView load start: $url');
      },
      onLoadStop: (ctrl, url) async {
        debugPrint('[Cortana] Load stop: $url');
        _updateLoadStatus('WebView load stop: $url');
        await _syncDiagnosticsVisibility();
        await _syncModelViewTransform();
        await _refreshLive2dDebugState();
      },
      onConsoleMessage: (ctrl, msg) {
        debugPrint('[Cortana Console] ${msg.message}');
        _appendLog('Console: ${msg.message}');
      },
      onReceivedError: (ctrl, request, error) {
        debugPrint(
          '[Cortana Error] ${error.type}: ${error.description} (${request.url})',
        );
        _updateLoadStatus('WebView error: ${error.description} (${request.url})');
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

  Future<void> _speak(String text) async {
    if (text.isEmpty || _speaking) return;

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
      child: Padding(
        padding: padding,
        child: child,
      ),
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
      return Text(
        '暂无可重播语音',
        style: Theme.of(context).textTheme.bodyMedium,
      );
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

  Widget _buildLogsContent() {
    if (_runtimeLogs.isEmpty) {
      return Text(
        '暂无日志',
        style: Theme.of(context).textTheme.bodyMedium,
      );
    }
    return ConstrainedBox(
      constraints: const BoxConstraints(maxHeight: 220),
      child: ListView.separated(
        shrinkWrap: true,
        padding: EdgeInsets.zero,
        itemCount: _runtimeLogs.length > 12 ? 12 : _runtimeLogs.length,
        separatorBuilder: (_, _) => const SizedBox(height: 6),
        itemBuilder: (context, index) => Text(
          _runtimeLogs[index],
          style: Theme.of(context).textTheme.bodySmall?.copyWith(
            fontFamily: 'monospace',
          ),
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final replayHistory = _combinedVoiceHistory();
    final screenWidth = MediaQuery.sizeOf(context).width;
    final overlayWidth = math.min(
      screenWidth < 640 ? screenWidth - 24 : screenWidth * 0.36,
      360.0,
    );
    final scaleText = _modelUserScale.toStringAsFixed(2);
    final offsetXText = _modelUserOffsetX.toStringAsFixed(2);
    final offsetYText = _modelUserOffsetY.toStringAsFixed(2);

    return Stack(
      children: [
        Positioned.fill(
          child:
              (!kIsWeb && defaultTargetPlatform == TargetPlatform.android)
              ? FutureBuilder<void>(
                  future: _androidLocalhostFuture,
                  builder: (context, snapshot) {
                    if (snapshot.hasError) {
                      return Center(
                        child: Text(
                          'Cortana localhost failed: ${snapshot.error}',
                        ),
                      );
                    }
                    if (snapshot.connectionState != ConnectionState.done) {
                      return const Center(
                        child: CircularProgressIndicator(),
                      );
                    }
                    return _buildWebView(
                      initialUrlRequest: URLRequest(
                        url: WebUri(
                          'http://localhost:$_localhostPort/$_cortanaLocalPath',
                        ),
                      ),
                    );
                  },
                )
              : _buildWebView(initialFile: _cortanaHtmlAsset),
        ),
        SafeArea(
          child: Stack(
            children: [
              Positioned(
                left: 12,
                top: 12,
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
                            subtitle: _live2dSummaryExpanded
                                ? '点击收起'
                                : '默认折叠，点击查看',
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
                            subtitle: _expressionActionsExpanded
                                ? '点击收起'
                                : '默认折叠，点击展开控制',
                            expanded: _expressionActionsExpanded,
                            onExpansionChanged: (expanded) {
                              setState(() {
                                _expressionActionsExpanded = expanded;
                              });
                            },
                            child: SingleChildScrollView(
                              scrollDirection: Axis.horizontal,
                              child: Row(
                                children: [
                                  for (final e in _expressions)
                                    Padding(
                                      padding: const EdgeInsets.only(right: 6),
                                      child: ActionChip(
                                        label: Text(e),
                                        onPressed: () => _callJS(
                                          "window.setExpression('$e')",
                                        ),
                                      ),
                                    ),
                                  const SizedBox(width: 8),
                                  for (final m in _motions)
                                    Padding(
                                      padding: const EdgeInsets.only(right: 6),
                                      child: ActionChip(
                                        label: Text(m),
                                        avatar: const Icon(
                                          Icons.directions_run,
                                          size: 14,
                                        ),
                                        onPressed: () => _callJS(
                                          "window.setMotion('${_normalizeMotion(m)}', 0)",
                                        ),
                                      ),
                                    ),
                                ],
                              ),
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
                            child: _buildReplayHistoryContent(
                              context,
                              replayHistory,
                            ),
                          ),
                          const SizedBox(height: 8),
                          _buildExpandableSectionCard(
                            storageKey: 'cortana-runtime-logs',
                            title: '显示日志',
                            subtitle: _showLogs
                                ? (_androidLoadStatus ?? '点击收起')
                                : '默认折叠，点击查看',
                            expanded: _showLogs,
                            onExpansionChanged: (expanded) {
                              setState(() {
                                _showLogs = expanded;
                              });
                              unawaited(_syncDiagnosticsVisibility());
                            },
                            child: _buildLogsContent(),
                          ),
                        ],
                      ),
                    ),
                  ),
                ),
              ),
              Positioned(
                left: 12,
                right: 12,
                bottom: 12,
                child: _buildSectionCard(
                  child: Row(
                    children: [
                      Expanded(
                        child: TextField(
                          controller: _textCtrl,
                          decoration: const InputDecoration(
                            hintText: '输入让 Cortana 说的话...',
                            isDense: true,
                            border: OutlineInputBorder(),
                            contentPadding: EdgeInsets.symmetric(
                              horizontal: 12,
                              vertical: 8,
                            ),
                          ),
                        ),
                      ),
                      const SizedBox(width: 8),
                      FilledButton.icon(
                        onPressed: _speaking
                            ? null
                            : () => _speak(_textCtrl.text.trim()),
                        icon: _speaking
                            ? const SizedBox(
                                width: 16,
                                height: 16,
                                child: CircularProgressIndicator(
                                  strokeWidth: 2,
                                ),
                              )
                            : const Icon(Icons.record_voice_over),
                        label: Text(_speaking ? '说话中' : '说话'),
                      ),
                    ],
                  ),
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }
}
