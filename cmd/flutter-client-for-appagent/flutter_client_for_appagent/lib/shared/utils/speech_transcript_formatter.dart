const String _hanCharClass = r'\u3400-\u4DBF\u4E00-\u9FFF\uF900-\uFAFF';
const String _cjkPunctuationClass = r'，。！？；：、（）《》〈〉【】「」『』“”‘’…—·';

final RegExp _hanCharRegex = RegExp('^[$_hanCharClass]\$');
final RegExp _cjkPunctuationRegex = RegExp('^[$_cjkPunctuationClass]\$');

bool _isHighSurrogate(int codeUnit) => codeUnit >= 0xD800 && codeUnit <= 0xDBFF;

bool _isLowSurrogate(int codeUnit) => codeUnit >= 0xDC00 && codeUnit <= 0xDFFF;

/// 移除孤立 UTF-16 代理项，避免 Flutter 文本渲染在 native 回传异常字符串时崩溃。
String sanitizeWellFormedUtf16(String raw) {
  if (raw.isEmpty) {
    return '';
  }

  final buffer = StringBuffer();
  var start = 0;
  var changed = false;

  for (var i = 0; i < raw.length; i++) {
    final codeUnit = raw.codeUnitAt(i);
    if (_isHighSurrogate(codeUnit)) {
      if (i + 1 < raw.length && _isLowSurrogate(raw.codeUnitAt(i + 1))) {
        i++;
        continue;
      }
      if (start < i) {
        buffer.write(raw.substring(start, i));
      }
      start = i + 1;
      changed = true;
      continue;
    }
    if (_isLowSurrogate(codeUnit)) {
      if (start < i) {
        buffer.write(raw.substring(start, i));
      }
      start = i + 1;
      changed = true;
    }
  }

  if (!changed) {
    return raw;
  }
  if (start < raw.length) {
    buffer.write(raw.substring(start));
  }
  return buffer.toString();
}

/// Vosk 和部分系统语音识别会把中文按词切开返回，这里补一层中文去空格。
String normalizeSpeechTranscript(String raw) {
  final normalizedWhitespace = sanitizeWellFormedUtf16(
    raw,
  ).replaceAll(RegExp(r'\s+'), ' ').trim();
  if (normalizedWhitespace.isEmpty) {
    return '';
  }

  final tokens = normalizedWhitespace.split(' ');
  final buffer = StringBuffer(tokens.first);

  for (final token in tokens.skip(1)) {
    if (_shouldMergeWithoutSpace(buffer.toString(), token)) {
      buffer.write(token);
      continue;
    }
    buffer.write(' ');
    buffer.write(token);
  }

  return buffer.toString();
}

bool _shouldMergeWithoutSpace(String previousToken, String currentToken) {
  if (previousToken.isEmpty || currentToken.isEmpty) {
    return false;
  }

  final previousChar = previousToken.substring(previousToken.length - 1);
  final currentChar = currentToken.substring(0, 1);
  final previousIsHan = _hanCharRegex.hasMatch(previousChar);
  final currentIsHan = _hanCharRegex.hasMatch(currentChar);
  final previousIsPunctuation = _cjkPunctuationRegex.hasMatch(previousChar);
  final currentIsPunctuation = _cjkPunctuationRegex.hasMatch(currentChar);

  return (previousIsHan && currentIsHan) ||
      (previousIsHan && currentIsPunctuation) ||
      (previousIsPunctuation && currentIsHan) ||
      (previousIsPunctuation && currentIsPunctuation);
}
