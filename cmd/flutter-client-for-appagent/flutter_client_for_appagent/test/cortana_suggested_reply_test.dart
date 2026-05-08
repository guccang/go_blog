import 'package:flutter_client_for_appagent/cortana_page.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('recognizes negative suggested replies', () {
    const replies = <CortanaSuggestedReply>[
      CortanaSuggestedReply(label: '否', message: '否'),
      CortanaSuggestedReply(label: '不想', message: '不想继续听'),
      CortanaSuggestedReply(label: '先不用', message: '先不用'),
      CortanaSuggestedReply(label: '不用', message: '不用'),
      CortanaSuggestedReply(label: 'No', message: 'No'),
      CortanaSuggestedReply(label: 'Later', message: '', kind: 'negative'),
    ];

    for (final reply in replies) {
      expect(reply.isNegativeAcknowledgement, true, reason: reply.label);
    }
  });

  test('does not treat positive or custom replies as negative', () {
    const replies = <CortanaSuggestedReply>[
      CortanaSuggestedReply(label: '想', message: '想继续听'),
      CortanaSuggestedReply(label: '需要', message: '需要'),
      CortanaSuggestedReply(label: '去看看', message: '去看看'),
      CortanaSuggestedReply(label: '其他', message: '', kind: 'custom'),
    ];

    for (final reply in replies) {
      expect(reply.isNegativeAcknowledgement, false, reason: reply.label);
    }
  });
}
