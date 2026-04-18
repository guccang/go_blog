import 'dart:async';
import 'dart:convert';
import 'package:audioplayers/audioplayers.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_inappwebview/flutter_inappwebview.dart';
import 'package:http/http.dart' as http;

class CortanaReplyPayload {
  const CortanaReplyPayload({
    required this.text,
    this.audioPath = '',
    this.audioFormat = '',
    this.actionPlan,
  });

  final String text;
  final String audioPath;
  final String audioFormat;
  final Map<String, dynamic>? actionPlan;

  bool get hasAudio => audioPath.trim().isNotEmpty;
}

class CortanaPage extends StatefulWidget {
  const CortanaPage({super.key, this.onSendMessage});

  final Future<CortanaReplyPayload> Function(String message)? onSendMessage;

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
  StreamSubscription<Duration>? _audioPositionSub;
  final List<Timer> _motionTimers = <Timer>[];
  bool _speaking = false;
  InAppLocalhostServer? _localhostServer;
  Future<void>? _androidLocalhostFuture;
  String? _androidLoadStatus;
  bool _logExpanded = false;

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
      _androidLocalhostFuture = _localhostServer!
          .start()
          .then((_) {
            if (mounted) {
              setState(() {
                _androidLoadStatus =
                    'Localhost ready: http://localhost:$_localhostPort/$_cortanaLocalPath';
              });
            }
          })
          .catchError((Object error) {
            if (mounted) {
              setState(() {
                _androidLoadStatus = 'Localhost start failed: $error';
              });
            }
            throw error;
          });
    }
  }

  @override
  void dispose() {
    _resetPlaybackEffects();
    _audio.dispose();
    _textCtrl.dispose();
    final localhostServer = _localhostServer;
    if (localhostServer != null) {
      unawaited(localhostServer.close());
    }
    super.dispose();
  }

  Future<void> _callJS(String js) async {
    try {
      final result = await _webCtrl?.evaluateJavascript(source: js);
      debugPrint('[Cortana JS Call] $js => $result');
    } catch (error, stackTrace) {
      debugPrint('[Cortana JS Call Error] $js => $error');
      debugPrint('$stackTrace');
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
            return {'ok': true};
          },
        );
      },
      onLoadStart: (ctrl, url) {
        debugPrint('[Cortana] Load start: $url');
        if (!kIsWeb && defaultTargetPlatform == TargetPlatform.android) {
          setState(() {
            _androidLoadStatus = 'WebView load start: $url';
          });
        }
      },
      onLoadStop: (ctrl, url) async {
        debugPrint('[Cortana] Load stop: $url');
        if (!kIsWeb && defaultTargetPlatform == TargetPlatform.android) {
          setState(() {
            _androidLoadStatus = 'WebView load stop: $url';
          });
        }
        final state = await ctrl.evaluateJavascript(
          source:
              'window.cortanaDebugState ? JSON.stringify(window.cortanaDebugState()) : "debug-state-unavailable";',
        );
        debugPrint('[Cortana] JS state after load: $state');
      },
      onConsoleMessage: (ctrl, msg) =>
          debugPrint('[Cortana Console] ${msg.message}'),
      onReceivedError: (ctrl, request, error) {
        debugPrint(
          '[Cortana Error] ${error.type}: ${error.description} (${request.url})',
        );
        if (!kIsWeb && defaultTargetPlatform == TargetPlatform.android) {
          setState(() {
            _androidLoadStatus =
                'WebView error: ${error.description} (${request.url})';
          });
        }
      },
      onReceivedHttpError: (ctrl, request, response) {
        debugPrint(
          '[Cortana HTTP Error] ${response.statusCode} ${response.reasonPhrase} (${request.url})',
        );
        if (!kIsWeb && defaultTargetPlatform == TargetPlatform.android) {
          setState(() {
            _androidLoadStatus =
                'HTTP ${response.statusCode} ${response.reasonPhrase} (${request.url})';
          });
        }
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

  Future<Uint8List> _synthesizeReplyAudio(String replyText) async {
    final ttsUrl = Uri.parse('http://blog.guccang.cn:10086/api/tts');
    final ttsResponse = await http
        .post(
          ttsUrl,
          headers: const <String, String>{
            'Content-Type': 'application/json',
            'Authorization': 'Bearer guccang@blog.guccang.cn',
          },
          body: jsonEncode(<String, dynamic>{
            'text': replyText,
            'provider': 'minimax',
            'voice': 'female-tianmei',
          }),
        )
        .timeout(const Duration(seconds: 30));

    if (ttsResponse.statusCode != 200) {
      throw Exception(
        'TTS failed: ${ttsResponse.statusCode} ${ttsResponse.body}',
      );
    }
    return Uint8List.fromList(ttsResponse.bodyBytes);
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
    final pulse =
        0.08 *
        (0.5 +
            ((startedAt.millisecondsSinceEpoch ~/ 80 +
                        (clamped * 1000).round()) %
                    7) /
                7);
    return (blended + pulse).clamp(0.12, 0.96);
  }

  void _startLipSyncLoop(String replyText) {
    _lipTimer?.cancel();
    _audioPositionSub?.cancel();
    _audioPositionSub = null;
    final estimatedDurationMs = _estimateSpeechDurationMs(replyText);
    final profile = _buildLipSyncProfile(replyText);
    final startedAt = DateTime.now();

    void pushLipSync(Duration position) {
      final progress = position.inMilliseconds / estimatedDurationMs;
      final amp = _lipAmplitudeForProgress(progress, profile, startedAt);
      _callJS('window.startLipSync($amp)');
    }

    _audioPositionSub = _audio.onPositionChanged.listen(pushLipSync);
    _lipTimer = Timer.periodic(const Duration(milliseconds: 120), (_) {
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
      final reply = await widget.onSendMessage!(text);
      final replyText = reply.text.trim();

      if (replyText.isEmpty) {
        throw Exception('LLM returned empty response');
      }

      debugPrint(
        '[Cortana LLM] User: $text, Reply: $replyText, audio=${reply.audioPath}',
      );

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
      if (expressionHoldMs > 0) {
        await _callJS(
          "window.setExpressionFor('$expression', $expressionHoldMs, '$fallbackExpression')",
        );
      } else {
        await _callJS("window.setExpression('$expression')");
      }

      final actions = actionPlan['actions'] as List<dynamic>? ?? [];
      _scheduleActions(actions);
      _startLipSyncLoop(replyText);

      if (reply.hasAudio) {
        await _audio.play(DeviceFileSource(reply.audioPath));
      } else {
        final audioBytes = await _synthesizeReplyAudio(replyText);
        debugPrint(
          '[Cortana TTS] Fallback synthesized ${audioBytes.length} bytes',
        );
        await _audio.play(BytesSource(audioBytes));
      }

      await _audio.onPlayerComplete.first;

      _resetPlaybackEffects();
      await _callJS('window.stopLipSync()');

      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Cortana: $replyText'),
            duration: const Duration(seconds: 3),
          ),
        );
      }
    } catch (e, stackTrace) {
      debugPrint('[Cortana Error] $e');
      debugPrint('$stackTrace');
      _resetPlaybackEffects();
      await _callJS('window.stopLipSync()');

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

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    return Column(
      children: [
        Expanded(
          child: Stack(
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
                          if (snapshot.connectionState !=
                              ConnectionState.done) {
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
              if (!kIsWeb &&
                  defaultTargetPlatform == TargetPlatform.android &&
                  _androidLoadStatus != null)
                Positioned(
                  left: 12,
                  right: 12,
                  top: 12,
                  child: GestureDetector(
                    onTap: () {
                      setState(() {
                        _logExpanded = !_logExpanded;
                      });
                    },
                    child: AnimatedContainer(
                      duration: const Duration(milliseconds: 200),
                      decoration: BoxDecoration(
                        color: Colors.black.withValues(alpha: 0.72),
                        borderRadius: BorderRadius.circular(8),
                      ),
                      child: Padding(
                        padding: const EdgeInsets.symmetric(
                          horizontal: 12,
                          vertical: 8,
                        ),
                        child: Row(
                          children: [
                            Icon(
                              _logExpanded
                                  ? Icons.expand_less
                                  : Icons.expand_more,
                              color: Colors.white,
                              size: 16,
                            ),
                            const SizedBox(width: 8),
                            Expanded(
                              child: Text(
                                _logExpanded
                                    ? _androidLoadStatus!
                                    : '日志 (点击展开)',
                                style: const TextStyle(
                                  color: Colors.white,
                                  fontSize: 12,
                                ),
                                maxLines: _logExpanded ? null : 1,
                                overflow: _logExpanded
                                    ? TextOverflow.visible
                                    : TextOverflow.ellipsis,
                              ),
                            ),
                          ],
                        ),
                      ),
                    ),
                  ),
                ),
            ],
          ),
        ),
        Container(
          color: cs.surface,
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              // 表情行
              SingleChildScrollView(
                scrollDirection: Axis.horizontal,
                child: Row(
                  children: [
                    for (final e in _expressions)
                      Padding(
                        padding: const EdgeInsets.only(right: 6),
                        child: ActionChip(
                          label: Text(e),
                          onPressed: () =>
                              _callJS("window.setExpression('$e')"),
                        ),
                      ),
                    const SizedBox(width: 8),
                    for (final m in _motions)
                      Padding(
                        padding: const EdgeInsets.only(right: 6),
                        child: ActionChip(
                          label: Text(m),
                          avatar: const Icon(Icons.directions_run, size: 14),
                          onPressed: () => _callJS(
                            "window.setMotion('${_normalizeMotion(m)}', 0)",
                          ),
                        ),
                      ),
                  ],
                ),
              ),
              const SizedBox(height: 8),
              // 说话输入行
              Row(
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
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : const Icon(Icons.record_voice_over),
                    label: Text(_speaking ? '说话中' : '说话'),
                  ),
                ],
              ),
            ],
          ),
        ),
      ],
    );
  }
}
