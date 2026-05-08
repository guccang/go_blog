import 'dart:collection';

import 'cortana_page.dart' show CortanaReplyPayload;

typedef CortanaBroadcastPlayer =
    void Function(CortanaReplyPayload payload, void Function() onFinished);

class CortanaBroadcastQueue {
  final ListQueue<CortanaReplyPayload> _pending =
      ListQueue<CortanaReplyPayload>();
  bool _playing = false;

  bool get isPlaying => _playing;
  bool get hasPending => _pending.isNotEmpty;
  int get pendingCount => _pending.length;

  void enqueue(CortanaReplyPayload payload, CortanaBroadcastPlayer player) {
    _pending.addLast(payload);
    _tryPlayNext(player);
  }

  void enqueueLatest(
    CortanaReplyPayload payload,
    CortanaBroadcastPlayer player,
  ) {
    if (_playing) {
      _pending
        ..clear()
        ..addLast(payload);
      return;
    }
    enqueue(payload, player);
  }

  void _tryPlayNext(CortanaBroadcastPlayer player) {
    if (_playing || _pending.isEmpty) {
      return;
    }

    _playing = true;
    final next = _pending.removeFirst();
    player(next, () {
      _playing = false;
      _tryPlayNext(player);
    });
  }
}
