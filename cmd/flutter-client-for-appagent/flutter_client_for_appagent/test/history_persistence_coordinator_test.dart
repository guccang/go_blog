import 'dart:async';

import 'package:flutter_client_for_appagent/main.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('ScopedHistoryPersistenceCoordinator', () {
    test(
      'coalesces rapid writes for the same scope and persists only latest',
      () async {
        final persistedScopes = <String>[];
        final gate = Completer<void>();
        var callCount = 0;

        late final ScopedHistoryPersistenceCoordinator coordinator;
        coordinator = ScopedHistoryPersistenceCoordinator((scopeKey) async {
          callCount++;
          if (callCount == 1) {
            await gate.future;
          }
          persistedScopes.add(scopeKey);
        });

        coordinator.schedule('direct');
        coordinator.schedule('direct');
        coordinator.schedule('direct');

        await Future<void>.delayed(Duration.zero);
        expect(callCount, 1);

        gate.complete();
        await Future<void>.delayed(Duration.zero);
        await Future<void>.delayed(Duration.zero);

        expect(persistedScopes, <String>['direct', 'direct']);
      },
    );

    test(
      'invalidate keeps later writes ordered after in-flight work',
      () async {
        final persistedScopes = <String>[];
        final gate = Completer<void>();

        late final ScopedHistoryPersistenceCoordinator coordinator;
        coordinator = ScopedHistoryPersistenceCoordinator((scopeKey) async {
          if (scopeKey == 'direct' && !gate.isCompleted) {
            await gate.future;
          }
          persistedScopes.add(scopeKey);
        });

        coordinator.schedule('direct');
        await Future<void>.delayed(Duration.zero);

        coordinator.invalidate();
        coordinator.schedule('direct');
        gate.complete();

        await Future<void>.delayed(Duration.zero);
        await Future<void>.delayed(Duration.zero);

        expect(persistedScopes, <String>['direct', 'direct']);
      },
    );
  });
}
