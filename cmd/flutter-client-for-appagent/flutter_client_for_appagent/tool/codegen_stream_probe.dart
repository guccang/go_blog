import 'dart:convert';
import 'dart:io';

import 'package:flutter_client_for_appagent/core/models/incoming_dedupe.dart';

const _sampleJsonl = '''
{"message_id":"codegen_stream:acp_probe","sequence":101,"user_id":"ztt","content":"tool start","message_type":"text","timestamp":1781833538000,"meta":{"origin":"codegen-stream","session_id":"acp_probe"}}
{"message_id":"codegen_stream:acp_probe","sequence":102,"user_id":"ztt","content":"tool progress","message_type":"text","timestamp":1781833539000,"meta":{"origin":"codegen-stream","session_id":"acp_probe"}}
{"message_id":"codegen_stream:acp_probe","sequence":103,"user_id":"ztt","content":"task complete","message_type":"text","timestamp":1781833540000,"meta":{"origin":"codegen-stream","session_id":"acp_probe","event":"task_complete"}}
''';

class ProbeResult {
  const ProbeResult({
    required this.total,
    required this.accepted,
    required this.droppedStaleSequence,
    required this.droppedDuplicateId,
    required this.acceptedCodegenStream,
    required this.lastSequence,
  });

  final int total;
  final int accepted;
  final int droppedStaleSequence;
  final int droppedDuplicateId;
  final int acceptedCodegenStream;
  final int lastSequence;

  bool satisfies({required int minCodegenUpdates}) {
    return acceptedCodegenStream >= minCodegenUpdates;
  }

  String summary() {
    return [
      'total=$total',
      'accepted=$accepted',
      'accepted_codegen_stream=$acceptedCodegenStream',
      'dropped_stale_sequence=$droppedStaleSequence',
      'dropped_duplicate_id=$droppedDuplicateId',
      'last_sequence=$lastSequence',
    ].join(' ');
  }
}

class _ProbeState {
  final seenMessageIds = <String>{};
  int lastSequence = 0;
  int total = 0;
  int accepted = 0;
  int droppedStaleSequence = 0;
  int droppedDuplicateId = 0;
  int acceptedCodegenStream = 0;

  void consume(Map<String, dynamic> envelope) {
    total += 1;
    final sequence = _readInt(envelope['sequence']);
    if (sequence > 0 && sequence <= lastSequence) {
      droppedStaleSequence += 1;
      return;
    }
    if (sequence > 0) {
      lastSequence = sequence;
    }

    final meta = _readMap(envelope['meta']);
    final origin = (meta['origin'] ?? '').toString();
    final messageId = (envelope['message_id'] ?? '').toString();
    if (shouldDedupeIncomingMessageId(
      origin: origin,
      messageId: messageId,
      seenMessageIds: seenMessageIds,
    )) {
      droppedDuplicateId += 1;
      return;
    }

    accepted += 1;
    if (origin.trim() == 'codegen-stream') {
      acceptedCodegenStream += 1;
      return;
    }
    if (messageId.trim().isNotEmpty) {
      seenMessageIds.add(messageId.trim());
    }
  }

  ProbeResult finish() {
    return ProbeResult(
      total: total,
      accepted: accepted,
      droppedStaleSequence: droppedStaleSequence,
      droppedDuplicateId: droppedDuplicateId,
      acceptedCodegenStream: acceptedCodegenStream,
      lastSequence: lastSequence,
    );
  }
}

ProbeResult runCodegenStreamProbe(String jsonl) {
  final state = _ProbeState();
  for (final line in const LineSplitter().convert(jsonl)) {
    final trimmed = line.trim();
    if (trimmed.isEmpty || trimmed.startsWith('#')) {
      continue;
    }
    final decoded = jsonDecode(trimmed);
    if (decoded is! Map<String, dynamic>) {
      throw FormatException('JSONL line must be an object: $trimmed');
    }
    state.consume(decoded);
  }
  return state.finish();
}

Future<int> main(List<String> args) async {
  final inputPath = _optionValue(args, '--input');
  final minCodegenUpdates =
      int.tryParse(_optionValue(args, '--min-codegen-updates') ?? '') ?? 2;
  final jsonl = inputPath == null
      ? _sampleJsonl
      : inputPath == '-'
      ? await stdin.transform(utf8.decoder).join()
      : await File(inputPath).readAsString(encoding: utf8);

  final result = runCodegenStreamProbe(jsonl);
  stdout.writeln(result.summary());
  if (!result.satisfies(minCodegenUpdates: minCodegenUpdates)) {
    stderr.writeln(
      'codegen stream probe failed: expected at least '
      '$minCodegenUpdates accepted codegen-stream updates',
    );
    return 1;
  }
  return 0;
}

String? _optionValue(List<String> args, String name) {
  for (var i = 0; i < args.length; i += 1) {
    final arg = args[i];
    if (arg == name && i + 1 < args.length) {
      return args[i + 1];
    }
    if (arg.startsWith('$name=')) {
      return arg.substring(name.length + 1);
    }
  }
  return null;
}

Map<String, dynamic> _readMap(Object? value) {
  if (value is Map<String, dynamic>) {
    return value;
  }
  if (value is Map) {
    return Map<String, dynamic>.from(value);
  }
  return const <String, dynamic>{};
}

int _readInt(Object? value) {
  if (value is int) {
    return value;
  }
  return int.tryParse('$value') ?? 0;
}
