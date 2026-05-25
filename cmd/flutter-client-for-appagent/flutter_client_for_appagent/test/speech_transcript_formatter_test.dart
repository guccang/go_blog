import 'package:flutter_client_for_appagent/speech_transcript_formatter.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('normalizeSpeechTranscript', () {
    test('removes spaces between Chinese words from local ASR output', () {
      expect(
        normalizeSpeechTranscript('你好 小 元宝 你 现在 在 做 什么 呢'),
        '你好小元宝你现在在做什么呢',
      );
    });

    test('removes spaces around Chinese punctuation', () {
      expect(normalizeSpeechTranscript('你好 ， 小 元宝 ！'), '你好，小元宝！');
    });

    test('keeps spaces in non-Chinese phrases', () {
      expect(normalizeSpeechTranscript('openai gpt 5'), 'openai gpt 5');
    });

    test('collapses repeated whitespace before formatting', () {
      expect(normalizeSpeechTranscript('  你好   小   元宝  '), '你好小元宝');
    });

    test('removes isolated utf16 surrogates from native speech text', () {
      final malformed = String.fromCharCodes(<int>[
        0xD800,
        0x4F60,
        0xDC00,
        0x597D,
      ]);

      expect(sanitizeWellFormedUtf16(malformed), '你好');
      expect(normalizeSpeechTranscript(malformed), '你好');
    });
  });
}
