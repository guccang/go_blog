const String _hanCharClass = r'\u3400-\u4DBF\u4E00-\u9FFF\uF900-\uFAFF';
const String _cjkPunctuationClass = r'，。！？；：、（）《》〈〉【】「」『』“”‘’…—·';

final RegExp _hanCharRegex = RegExp('^[$_hanCharClass]\$');
final RegExp _cjkPunctuationRegex = RegExp('^[$_cjkPunctuationClass]\$');

/// Vosk 和部分系统语音识别会把中文按词切开返回，这里补一层中文去空格。
String normalizeSpeechTranscript(String raw) {
  final normalizedWhitespace = raw.replaceAll(RegExp(r'\s+'), ' ').trim();
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
