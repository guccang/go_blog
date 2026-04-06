import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_client_for_appagent/main.dart';

void main() {
  group('resolveVoiceGestureAction', () {
    test('uses upper-left drag to cancel', () {
      expect(
        resolveVoiceGestureAction(const Offset(-36, -56)),
        VoiceGestureAction.cancel,
      );
    });

    test('uses upper-right drag to transcribe', () {
      expect(
        resolveVoiceGestureAction(const Offset(36, -56)),
        VoiceGestureAction.transcribe,
      );
    });

    test('uses strong left swipe to cancel even without enough upward drag', () {
      expect(
        resolveVoiceGestureAction(const Offset(-84, -8)),
        VoiceGestureAction.cancel,
      );
    });

    test(
      'uses strong right swipe to transcribe even without enough upward drag',
      () {
        expect(
          resolveVoiceGestureAction(const Offset(84, 8)),
          VoiceGestureAction.transcribe,
        );
      },
    );

    test('keeps send as default action for small drag', () {
      expect(
        resolveVoiceGestureAction(const Offset(12, -16)),
        VoiceGestureAction.sendAudio,
      );
    });
  });
}
