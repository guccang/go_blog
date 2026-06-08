import 'package:flutter_client_for_appagent/features/codegen/models.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('returns the resume identifier used by each supported coding tool', () {
    expect(codegenResumeIdentifierForTool('codex'), 'resume');
    expect(codegenResumeIdentifierForTool('claudecode'), '-c');
    expect(codegenResumeIdentifierForTool(' CODEX '), 'resume');
  });

  test('returns empty identifier for unsupported tools', () {
    expect(codegenResumeIdentifierForTool('opencode'), isEmpty);
    expect(codegenResumeIdentifierForTool(''), isEmpty);
  });
}
