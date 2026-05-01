import 'package:flutter_client_for_appagent/cortana_broadcast_queue.dart';
import 'package:flutter_client_for_appagent/cortana_page.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('broadcast queue starts immediately', () {
    final queue = CortanaBroadcastQueue();
    final played = <String>[];

    queue.enqueue(const CortanaReplyPayload(text: 'first'), (
      payload,
      onFinished,
    ) {
      played.add(payload.text);
    });

    expect(played, <String>['first']);
    expect(queue.isPlaying, true);
  });

  test('broadcast queue preserves order while current playback is active', () {
    final queue = CortanaBroadcastQueue();
    final played = <String>[];
    final completions = <void Function()>[];

    void player(CortanaReplyPayload payload, void Function() onFinished) {
      played.add(payload.text);
      completions.add(onFinished);
    }

    queue.enqueue(const CortanaReplyPayload(text: 'first'), player);
    queue.enqueue(const CortanaReplyPayload(text: 'second'), player);

    expect(played, <String>['first']);
    expect(queue.pendingCount, 1);

    completions.removeAt(0)();

    expect(played, <String>['first', 'second']);
    expect(queue.pendingCount, 0);

    completions.removeAt(0)();
    expect(queue.isPlaying, false);
  });
}
