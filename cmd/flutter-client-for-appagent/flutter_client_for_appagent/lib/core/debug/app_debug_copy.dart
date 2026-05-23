part of '../../main.dart';

class DebugCopyLogSection {
  const DebugCopyLogSection({
    required this.name,
    required this.sourceType,
    required this.content,
    this.limit = 'latest 100 lines',
    this.order = 'oldest_to_newest',
    this.fetchStatus = 'ok',
    this.error = '',
  });

  final String name;
  final String sourceType;
  final String content;
  final String limit;
  final String order;
  final String fetchStatus;
  final String error;
}

class AppDebugCopyBuilder {
  const AppDebugCopyBuilder._();

  static String build({
    required Map<String, dynamic> bundle,
    required Map<String, dynamic> issue,
    required Map<String, dynamic> appState,
    required List<DebugCopyLogSection> sections,
  }) {
    final encoder = const JsonEncoder.withIndent('  ');
    final out = StringBuffer()
      ..writeln('# Flutter App Debug Copy')
      ..writeln()
      ..writeln('## Debug Bundle')
      ..writeln('debug_id: ${_scalar(bundle['debug_id'])}')
      ..writeln('debug_path: ${_scalar(bundle['debug_path'])}')
      ..writeln('created_at: ${_scalar(bundle['created_at'])}')
      ..writeln('platform: ${_scalar(bundle['platform'])}')
      ..writeln('app_version: ${_scalar(bundle['app_version'])}')
      ..writeln()
      ..writeln('## Issue')
      ..writeln(_fence('text', _issueText(issue)))
      ..writeln()
      ..writeln('## App State')
      ..writeln(_fence('json', encoder.convert(appState)))
      ..writeln();

    for (final section in sections) {
      out
        ..writeln('## Log Source: ${section.name}')
        ..writeln('source_type: ${section.sourceType}')
        ..writeln('limit: ${section.limit}')
        ..writeln('order: ${section.order}')
        ..writeln('fetch_status: ${section.fetchStatus}');
      if (section.error.trim().isNotEmpty) {
        out.writeln('error: ${section.error.trim()}');
      }
      out
        ..writeln()
        ..writeln(
          _fence(_languageForSection(section), section.content.trimRight()),
        )
        ..writeln();
    }

    out
      ..writeln('## Suggested Entry Files')
      ..writeln(
        '- cmd/flutter-client-for-appagent/flutter_client_for_appagent/lib/features/chat/chat_page_core.dart',
      )
      ..writeln(
        '- cmd/flutter-client-for-appagent/flutter_client_for_appagent/lib/core/platform/app_platform_services.dart',
      )
      ..writeln(
        '- cmd/flutter-client-for-appagent/flutter_client_for_appagent/android/app/src/main/kotlin/com/example/flutter_client_for_appagent/MainActivity.kt',
      )
      ..writeln()
      ..writeln('## Constraints')
      ..writeln('- 所有文件读写必须 UTF-8 无 BOM。')
      ..writeln('- 禁止执行 flutter build apk 或任何 APK 打包命令。')
      ..writeln(
        '- 验证优先级：dart analyze -> flutter analyze -> dart format --set-exit-if-changed . -> flutter test。',
      );
    return redactDebugCopyText(out.toString());
  }

  static String _issueText(Map<String, dynamic> issue) {
    final lines = <String>[];
    final title = _scalar(issue['title']);
    if (title.isNotEmpty) {
      lines.add('Title: $title');
    }
    final description = _scalar(issue['user_description']);
    if (description.isNotEmpty) {
      lines.add('User description:');
      lines.add(description);
    }
    final expected = _scalar(issue['expected']);
    if (expected.isNotEmpty) {
      lines.add('Expected: $expected');
    }
    final actual = _scalar(issue['actual']);
    if (actual.isNotEmpty) {
      lines.add('Actual: $actual');
    }
    final steps = issue['repro_steps'];
    if (steps is Iterable && steps.isNotEmpty) {
      lines.add('Repro steps:');
      var index = 1;
      for (final step in steps) {
        lines.add('$index. $step');
        index++;
      }
    }
    return lines.join('\n');
  }

  static String _languageForSection(DebugCopyLogSection section) {
    final lower = section.name.toLowerCase();
    if (lower.endsWith('.jsonl')) {
      return 'jsonl';
    }
    if (lower.endsWith('.json')) {
      return 'json';
    }
    return 'text';
  }

  static String _fence(String language, String value) {
    return '~~~$language\n${value.trimRight()}\n~~~';
  }

  static String _scalar(dynamic value) {
    return (value ?? '').toString().trim();
  }
}

String redactDebugCopyText(String input) {
  var text = input;
  text = text.replaceAllMapped(
    RegExp(
      r'\b(authorization|cookie|set-cookie|x-api-key)\b\s*[:=]\s*[^\n]+',
      caseSensitive: false,
    ),
    (match) => '${match.group(1)}: <redacted>',
  );
  text = text.replaceAllMapped(
    RegExp(
      r'"(password|token|access_token|refresh_token|session_token|secret|private_key)"\s*:\s*"[^"]*"',
      caseSensitive: false,
    ),
    (match) => '"${match.group(1)}":"<redacted>"',
  );
  text = text.replaceAllMapped(
    RegExp(
      r'\b(token|access_token|refresh_token|session_token|password)=([^&\s]+)',
      caseSensitive: false,
    ),
    (match) => '${match.group(1)}=<redacted>',
  );
  return text;
}

extension _ChatPageStateDebugCopy on _ChatPageState {
  Future<void> _copyFullDebugContext() async {
    final issue = _defaultDebugCopyIssue();
    final appState = _buildDebugCopyAppState();
    final clientLogs = _recentFlutterClientLogLines(limit: 100);
    final llmEvents = _recentLlmDebugEventMaps(limit: 100);
    final voiceEvents = await AppDebugRecorder.instance.recentEvents(
      'voice_wake',
      limit: 100,
    );
    final crashEvents = await AppDebugRecorder.instance.recentEvents(
      'crash',
      limit: 100,
    );
    final nativeCrashJsonl = await _readNativeCrashJsonl();

    final sections = <DebugCopyLogSection>[
      DebugCopyLogSection(
        name: 'flutter_client.log',
        sourceType: 'flutter_memory_log',
        content: clientLogs.join('\n'),
      ),
      DebugCopyLogSection(
        name: 'voice_wake.jsonl',
        sourceType: 'flutter_structured_trace',
        limit: 'latest 100 events',
        content: _jsonLines(voiceEvents),
      ),
      DebugCopyLogSection(
        name: 'crash.jsonl',
        sourceType: 'flutter_and_native_crash_trace',
        limit: 'latest 100 events',
        content: _mergeCrashJsonl(crashEvents, nativeCrashJsonl),
      ),
      DebugCopyLogSection(
        name: 'llm_debug_events.jsonl',
        sourceType: 'app_llm_debug_events',
        limit: 'latest 100 events',
        content: _jsonLines(llmEvents),
      ),
    ];

    var bundle = <String, dynamic>{
      'debug_id': '',
      'debug_path': '',
      'created_at': DateTime.now().toIso8601String(),
      'platform': _platformLabel(),
      'app_version': appVersion,
    };

    if (_clientConfig == null || _sessionToken.trim().isEmpty) {
      sections.add(
        const DebugCopyLogSection(
          name: 'agent_logs',
          sourceType: 'server_log',
          content: '',
          fetchStatus: 'failed',
          error: 'login required or app-agent config missing',
        ),
      );
    } else {
      try {
        final resp = await _runAuthed('Create debug copy bundle', (client) {
          return client.createDebugBundle(
            issue: issue,
            appState: appState,
            timeline: <Map<String, dynamic>>[
              ...voiceEvents,
              ...crashEvents,
              ...llmEvents,
            ],
            clientLogs: clientLogs,
          );
        });
        bundle = <String, dynamic>{
          'debug_id': (resp['debug_id'] ?? '').toString(),
          'debug_path': (resp['bundle_path'] ?? '').toString(),
          'created_at': DateTime.now().toIso8601String(),
          'platform': _platformLabel(),
          'app_version': appVersion,
        };
        sections.addAll(await _fetchAgentLogSections());
      } catch (err) {
        sections.add(
          DebugCopyLogSection(
            name: 'agent_logs',
            sourceType: 'server_log',
            content: '',
            fetchStatus: 'failed',
            error: _describeRequestError(err, operation: 'Fetch agent logs'),
          ),
        );
      }
    }

    final text = AppDebugCopyBuilder.build(
      bundle: bundle,
      issue: issue,
      appState: appState,
      sections: sections,
    );
    await _copyText('完整调试上下文', text);
    addFlutterClientLog('Debug Copy 已复制，长度=${text.length}');
  }

  Map<String, dynamic> _defaultDebugCopyIssue() {
    return <String, dynamic>{
      'title': 'Flutter App Vosk 语音唤醒调试',
      'user_description':
          '1. 40M Vosk 模型识别不准：说“嗨”返回“还”，说“元宝”返回“院报”等。\n'
          '2. 如果命中“嗨”，App 直接崩溃退出。',
      'expected': '识别结果可解释，命中唤醒词后 App 不崩溃。',
      'actual': _lastCortanaWakeTranscript.isEmpty
          ? '等待最近语音唤醒日志定位。'
          : '最近语音识别: $_lastCortanaWakeTranscript',
      'repro_steps': <String>[
        '打开 App 并启用 Cortana 语音唤醒。',
        '使用本地 Vosk 模型说“嗨”或“元宝”。',
        '如果 App 崩溃，重启后进入调试页执行复制完整调试上下文。',
      ],
    };
  }

  Map<String, dynamic> _buildDebugCopyAppState() {
    final scopeKey = _currentGroupId.isEmpty
        ? 'direct'
        : _groupScopeKey(_currentGroupId);
    return <String, dynamic>{
      'root_tab': _rootTab.name,
      'connection': _connectionLabel,
      'websocket_connected': _connected,
      'current_group_id': _currentGroupId,
      'message_count': _historyByScope[scopeKey]?.length ?? 0,
      'llm_debug_event_count': _llmDebugEvents.length,
      'cortana_mode': _cortanaFloatingMode.name,
      'cortana_enabled': _cortanaEnabled,
      'voice_wake_enabled': _cortanaVoiceWakeEnabled,
      'wake_phrase': _cortanaWakePhrase,
      'use_local_vosk': _useLocalVosk,
      'speech_ready': _speechReady,
      'system_speech_ready': _systemSpeechReady,
      'persistent_vosk_wake_listening': _persistentVoskWakeListening,
      'cortana_wake_listening': _cortanaWakeListening,
      'cortana_wake_handling': _cortanaWakeHandling,
      'cortana_wake_awaiting_command': _cortanaWakeAwaitingCommand,
      'last_cortana_wake_transcript': _lastCortanaWakeTranscript,
      'base_url': _clientConfig?.baseUrl ?? '',
      'platform': _platformLabel(),
      'app_version': appVersion,
    };
  }

  List<Map<String, dynamic>> _recentLlmDebugEventMaps({int limit = 100}) {
    return _llmDebugEvents.reversed
        .take(limit)
        .map(
          (event) => <String, dynamic>{
            'time': event.timestamp.toIso8601String(),
            'category': 'llm',
            'event': event.event,
            'label': event.label,
            'message': event.content,
            'payload': event.payload,
          },
        )
        .toList(growable: false)
        .reversed
        .toList(growable: false);
  }

  Future<List<DebugCopyLogSection>> _fetchAgentLogSections() async {
    return _runAuthed('Fetch agent logs', (client) async {
      final sources = await client.listLogSources();
      if (sources.isEmpty) {
        return const <DebugCopyLogSection>[
          DebugCopyLogSection(
            name: 'agent_logs',
            sourceType: 'server_log',
            content: '',
            fetchStatus: 'failed',
            error: 'no log sources configured',
          ),
        ];
      }
      final sections = <DebugCopyLogSection>[];
      for (final source in sources) {
        try {
          final log = await client.readLogContent(
            source: source.name,
            lines: 100,
          );
          sections.add(
            DebugCopyLogSection(
              name: log.file.isEmpty
                  ? '${source.name}.log'
                  : '${source.name}/${log.file}',
              sourceType: 'server_log',
              content: log.content,
              limit: 'latest 100 lines',
              fetchStatus: 'ok',
            ),
          );
        } catch (err) {
          sections.add(
            DebugCopyLogSection(
              name: '${source.name}.log',
              sourceType: 'server_log',
              content: '',
              fetchStatus: 'failed',
              error: _describeRequestError(err, operation: source.name),
            ),
          );
        }
      }
      return sections;
    });
  }

  Future<String> _readNativeCrashJsonl() async {
    if (!_isAndroidHost) {
      return '';
    }
    try {
      return await _voskTranscriber.readNativeDebugTrace('crash');
    } catch (err) {
      debugPrint('Read native crash trace failed: $err');
      return '';
    }
  }

  static String _jsonLines(List<Map<String, dynamic>> events) {
    return events.map(jsonEncode).join('\n');
  }

  static String _mergeCrashJsonl(
    List<Map<String, dynamic>> crashEvents,
    String nativeCrashJsonl,
  ) {
    final lines = <String>[
      ...crashEvents.map(jsonEncode),
      ...nativeCrashJsonl
          .split('\n')
          .map((line) => line.trim())
          .where((line) => line.isNotEmpty),
    ];
    if (lines.length <= 100) {
      return lines.join('\n');
    }
    return lines.sublist(lines.length - 100).join('\n');
  }
}
