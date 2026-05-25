import 'package:flutter_client_for_appagent/sherpa_keyword_tokenizer.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('builds ppinyin keyword buffer for Chinese wake phrase', () {
    final tokenizer = SherpaKeywordTokenizer(<String>{
      'x',
      'iǎo',
      'ài',
      't',
      'óng',
      'ué',
    });

    final result = tokenizer.buildKeywordBuffer(<String>['小爱同学']);

    expect(result.keywordBuffer, contains('x iǎo ài t óng x ué'));
    expect(result.keywordBuffer, contains('@小爱同学'));
    expect(result.acceptedAliases, contains('小爱同学'));
  });

  test('expands Cortana to Chinese aliases before tokenization', () {
    final aliases = SherpaKeywordTokenizer.expandChineseAliases(<String>[
      '嗨 Cortana',
    ]);

    expect(aliases, contains('嗨 科塔娜'));
    expect(aliases, contains('嗨科塔娜'));
    expect(aliases, contains('小娜'));
  });

  test('rejects unsupported English-only wake phrase', () {
    final tokenizer = SherpaKeywordTokenizer(<String>{'h', 'āi'});

    expect(
      () => tokenizer.buildKeywordBuffer(<String>['hello assistant']),
      throwsFormatException,
    );
  });
}
