import 'dart:convert';
import 'dart:io';

import 'package:pinyin/pinyin.dart';

class SherpaKeywordBuildResult {
  const SherpaKeywordBuildResult({
    required this.keywordBuffer,
    required this.acceptedAliases,
    required this.rejectedAliases,
  });

  final String keywordBuffer;
  final List<String> acceptedAliases;
  final Map<String, String> rejectedAliases;
}

class SherpaKeywordTokenizer {
  SherpaKeywordTokenizer(this.tokenSet);

  final Set<String> tokenSet;

  static Future<SherpaKeywordTokenizer> fromTokensFile(String path) async {
    final tokens = <String>{};
    final lines = await File(path).readAsLines(encoding: utf8);
    for (final line in lines) {
      final trimmed = line.trim();
      if (trimmed.isEmpty) {
        continue;
      }
      final parts = trimmed.split(RegExp(r'\s+'));
      final token = int.tryParse(parts.first) != null && parts.length > 1
          ? parts[1]
          : parts.first;
      if (token.isNotEmpty) {
        tokens.add(token);
      }
    }
    return SherpaKeywordTokenizer(tokens);
  }

  SherpaKeywordBuildResult buildKeywordBuffer(
    Iterable<String> phrases, {
    double score = 2.0,
    double threshold = 0.25,
  }) {
    final lines = <String>[];
    final accepted = <String>[];
    final rejected = <String, String>{};
    final seen = <String>{};

    for (final phrase in expandChineseAliases(phrases)) {
      final normalized = phrase.trim();
      if (normalized.isEmpty || !seen.add(normalized)) {
        continue;
      }
      final tokens = _tokenizeChinesePhrase(normalized);
      if (tokens == null || tokens.isEmpty) {
        rejected[normalized] = 'unsupported_non_chinese_or_empty';
        continue;
      }
      final missing = tokens
          .where((token) => !tokenSet.contains(token))
          .toSet();
      if (missing.isNotEmpty) {
        rejected[normalized] = 'missing_tokens:${missing.join(',')}';
        continue;
      }
      final label = _keywordLabel(normalized);
      lines.add('${tokens.join(' ')} :$score #$threshold @$label');
      accepted.add(normalized);
    }

    if (lines.isEmpty) {
      throw FormatException('唤醒词无法转换为当前 Sherpa KWS 模型支持的中文拼音 tokens。');
    }

    return SherpaKeywordBuildResult(
      keywordBuffer: lines.join('\n'),
      acceptedAliases: accepted,
      rejectedAliases: rejected,
    );
  }

  static Set<String> expandChineseAliases(Iterable<String> phrases) {
    final aliases = <String>{
      ...phrases,
      '嗨 Cortana',
      '嘿 Cortana',
      '你好 Cortana',
      '嗨 科塔娜',
      '嘿 科塔娜',
      '你好 科塔娜',
      '科塔娜',
      '小娜',
      '嗨 小娜',
      '你好 小娜',
    };
    final expanded = <String>{};
    for (final alias in aliases) {
      final trimmed = alias.trim();
      if (trimmed.isEmpty) {
        continue;
      }
      expanded.add(trimmed);
      final chineseCortana = trimmed.replaceAll(
        RegExp(r'cortana', caseSensitive: false),
        '科塔娜',
      );
      expanded.add(chineseCortana);
      expanded.add(chineseCortana.replaceAll(RegExp(r'\s+'), ''));
    }
    return expanded.where((alias) => alias.trim().isNotEmpty).toSet();
  }

  List<String>? _tokenizeChinesePhrase(String phrase) {
    final tokens = <String>[];
    for (final rune in phrase.runes) {
      final char = String.fromCharCode(rune);
      if (_isIgnoredPunctuation(char)) {
        continue;
      }
      if (!_isChineseRune(rune)) {
        return null;
      }
      final pinyin = _pinyinForChar(char);
      if (pinyin == null || pinyin.isEmpty) {
        return null;
      }
      tokens.addAll(_splitPinyinSyllable(pinyin));
    }
    return tokens;
  }

  String? _pinyinForChar(String char) {
    try {
      final pinyin = PinyinHelper.getPinyin(
        char,
        separator: ' ',
        format: PinyinFormat.WITH_TONE_MARK,
      ).trim();
      if (pinyin.isEmpty || pinyin.contains(' ')) {
        return null;
      }
      return pinyin;
    } catch (_) {
      return null;
    }
  }

  List<String> _splitPinyinSyllable(String syllable) {
    const initials = <String>[
      'zh',
      'ch',
      'sh',
      'b',
      'p',
      'm',
      'f',
      'd',
      't',
      'n',
      'l',
      'g',
      'k',
      'h',
      'j',
      'q',
      'x',
      'r',
      'z',
      'c',
      's',
      'y',
      'w',
    ];
    for (final initial in initials) {
      if (!syllable.startsWith(initial)) {
        continue;
      }
      final finalPart = syllable.substring(initial.length);
      if (finalPart.isNotEmpty) {
        return <String>[initial, finalPart];
      }
    }
    return <String>[syllable];
  }

  static bool _isChineseRune(int rune) {
    return (rune >= 0x3400 && rune <= 0x4DBF) ||
        (rune >= 0x4E00 && rune <= 0x9FFF) ||
        (rune >= 0x20000 && rune <= 0x2A6DF) ||
        (rune >= 0x2A700 && rune <= 0x2B73F) ||
        (rune >= 0x2B740 && rune <= 0x2B81F) ||
        (rune >= 0x2B820 && rune <= 0x2CEAF);
  }

  static bool _isIgnoredPunctuation(String char) {
    return RegExp(r"""^[\s,，.。!！?？、:：;；"“”'‘’\-—_]+$""").hasMatch(char);
  }

  static String _keywordLabel(String phrase) {
    return phrase
        .replaceAll(RegExp(r'\s+'), '')
        .replaceAll(RegExp(r"""[,，.。!！?？、:：;；"“”'‘’\-—_]+"""), '');
  }
}
