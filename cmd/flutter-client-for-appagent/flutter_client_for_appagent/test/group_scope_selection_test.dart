import 'package:flutter_client_for_appagent/main.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('resolvePreferredGroupId', () {
    const soloGroup = GroupInfo(
      id: 'team-alpha',
      members: <String>['demo-user'],
      createdAt: DateTime(2026, 1, 1),
    );

    test('keeps direct scope during background refresh for a single group', () {
      final selected = resolvePreferredGroupId(
        const <GroupInfo>[soloGroup],
        allowImplicitSingleSelection: false,
      );

      expect(selected, isEmpty);
    });

    test('retains explicit group selection when the group still exists', () {
      final selected = resolvePreferredGroupId(
        const <GroupInfo>[soloGroup],
        preferredGroupId: 'team-alpha',
        allowImplicitSingleSelection: false,
      );

      expect(selected, 'team-alpha');
    });

    test('still supports single-group auto selection for explicit group entry', () {
      final selected = resolvePreferredGroupId(const <GroupInfo>[soloGroup]);

      expect(selected, 'team-alpha');
    });
  });
}
