import 'package:flutter_client_for_appagent/main.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('debug copy includes log bodies and redacts sensitive values', () {
    final text = AppDebugCopyBuilder.build(
      bundle: <String, dynamic>{
        'debug_id': 'dbg_1',
        'debug_path': '/tmp/dbg_1',
        'created_at': '2026-05-21T15:30:00+08:00',
        'platform': 'android',
        'app_version': '1.0.0+1',
      },
      issue: <String, dynamic>{
        'title': 'Sherpa issue',
        'user_description': '说嗨返回还',
      },
      appState: <String, dynamic>{
        'voice_wake_enabled': true,
        'token': 'secret-token',
      },
      sections: const <DebugCopyLogSection>[
        DebugCopyLogSection(
          name: 'flutter_client.log',
          sourceType: 'flutter_memory_log',
          content: 'Authorization: Bearer secret\nline 2',
        ),
        DebugCopyLogSection(
          name: 'voice_wake.jsonl',
          sourceType: 'flutter_structured_trace',
          limit: 'latest 100 events',
          content: '{"text":"还","raw_payload":"{\\"partial\\":\\"还\\"}"}',
        ),
      ],
    );

    expect(text, contains('## Log Source: flutter_client.log'));
    expect(text, contains('line 2'));
    expect(text, contains('## Log Source: voice_wake.jsonl'));
    expect(text, contains('"text":"还"'));
    expect(text, isNot(contains('Bearer secret')));
    expect(text, isNot(contains('secret-token')));
    expect(text, contains('<redacted>'));
  });
}
