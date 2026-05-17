import 'package:flutter_client_for_appagent/main.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('chat message persistence removes embedded media base64 payloads', () {
    final message = ChatMessage(
      content: 'hello',
      direction: MessageDirection.incoming,
      timestamp: DateTime.fromMillisecondsSinceEpoch(0),
      meta: <String, dynamic>{
        'audio_base64': 'audio',
        'cortana_audio_base64': 'cortana-audio',
        'image_base64': 'image',
        'video_base64': 'video',
        'keep': 'value',
      },
    );

    final persisted = message.toJson();
    final meta = persisted['meta'] as Map<String, dynamic>;

    expect(meta.containsKey('audio_base64'), isFalse);
    expect(meta.containsKey('cortana_audio_base64'), isFalse);
    expect(meta.containsKey('image_base64'), isFalse);
    expect(meta.containsKey('video_base64'), isFalse);
    expect(meta['keep'], 'value');
  });
}
