import 'package:flutter_test/flutter_test.dart';

import '../tool/codegen_stream_probe.dart';

void main() {
  test(
    'probe accepts repeated codegen stream message id with newer sequence',
    () {
      final result = runCodegenStreamProbe('''
{"message_id":"codegen_stream:acp_test","sequence":10,"user_id":"ztt","content":"start","message_type":"text","timestamp":1781833538000,"meta":{"origin":"codegen-stream","session_id":"acp_test"}}
{"message_id":"codegen_stream:acp_test","sequence":11,"user_id":"ztt","content":"progress","message_type":"text","timestamp":1781833539000,"meta":{"origin":"codegen-stream","session_id":"acp_test"}}
{"message_id":"codegen_stream:acp_test","sequence":12,"user_id":"ztt","content":"done","message_type":"text","timestamp":1781833540000,"meta":{"origin":"codegen-stream","session_id":"acp_test","event":"task_complete"}}
''');

      expect(result.acceptedCodegenStream, 3);
      expect(result.droppedDuplicateId, 0);
      expect(result.satisfies(minCodegenUpdates: 3), isTrue);
    },
  );

  test('probe still dedupes normal repeated message ids', () {
    final result = runCodegenStreamProbe('''
{"message_id":"normal_1","sequence":20,"user_id":"ztt","content":"first","message_type":"text","timestamp":1781833538000,"meta":{"origin":"app-process"}}
{"message_id":"normal_1","sequence":21,"user_id":"ztt","content":"duplicate","message_type":"text","timestamp":1781833539000,"meta":{"origin":"app-process"}}
''');

    expect(result.accepted, 1);
    expect(result.droppedDuplicateId, 1);
  });

  test('probe rejects stale sequence before message id dedupe', () {
    final result = runCodegenStreamProbe('''
{"message_id":"codegen_stream:acp_test","sequence":30,"user_id":"ztt","content":"new","message_type":"text","timestamp":1781833538000,"meta":{"origin":"codegen-stream","session_id":"acp_test"}}
{"message_id":"codegen_stream:acp_test","sequence":29,"user_id":"ztt","content":"stale","message_type":"text","timestamp":1781833539000,"meta":{"origin":"codegen-stream","session_id":"acp_test"}}
''');

    expect(result.acceptedCodegenStream, 1);
    expect(result.droppedStaleSequence, 1);
  });
}
