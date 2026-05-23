import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_client_for_appagent/main.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  const channel = MethodChannel('com.example.flutter_client_for_appagent/vosk');

  tearDown(() {
    channel.setMethodCallHandler(null);
  });

  test('wake word event handler errors are contained', () async {
    await AppDebugRecorder.instance.clearForTest();
    final transcriber = VoskTranscriber();
    transcriber.setWakeWordEventHandler((event) async {
      throw StateError('boom');
    });

    ByteData? response;
    await TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .handlePlatformMessage(
          channel.name,
          channel.codec.encodeMethodCall(
            const MethodCall('wakeWordEvent', <String, dynamic>{
              'type': 'partial',
              'payload': '{"partial":"嗨 Cortana"}',
            }),
          ),
          (data) => response = data,
        );

    expect(() => channel.codec.decodeEnvelope(response!), returnsNormally);

    final voiceEvents = await AppDebugRecorder.instance.recentEvents(
      'voice_wake',
      limit: 100,
    );
    expect(voiceEvents, isNotEmpty);
    expect(
      voiceEvents.last['data'],
      containsPair('raw_payload', '{"partial":"嗨 Cortana"}'),
    );

    final channelErrors = await AppDebugRecorder.instance.recentEvents(
      'method_channel',
      limit: 100,
    );
    expect(channelErrors, isNotEmpty);
    expect(channelErrors.last['message'], contains('boom'));
  });
}
