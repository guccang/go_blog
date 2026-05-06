import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_client_for_appagent/cortana_page.dart';

void main() {
  test('copies Live2D texture subdirectories into web runtime root', () async {
    final tempDir = await Directory.systemTemp.createTemp(
      'cortana-web-runtime-copy-',
    );
    addTearDown(() async {
      if (await tempDir.exists()) {
        await tempDir.delete(recursive: true);
      }
    });

    final source = Directory('${tempDir.path}${Platform.pathSeparator}source');
    final destination = Directory(
      '${tempDir.path}${Platform.pathSeparator}destination',
    );
    await source.create(recursive: true);
    await destination.create(recursive: true);

    await File(
      '${source.path}${Platform.pathSeparator}baimeimo.model3.json',
    ).writeAsString('{}', encoding: utf8);
    await File(
      '${source.path}${Platform.pathSeparator}baimeimo.4096'
      '${Platform.pathSeparator}texture_00.png',
    ).create(recursive: true);

    await copyCortanaDirectoryForWebRuntime(source, destination);

    expect(
      await File(
        '${destination.path}${Platform.pathSeparator}baimeimo.model3.json',
      ).exists(),
      isTrue,
    );
    expect(
      await File(
        '${destination.path}${Platform.pathSeparator}baimeimo.4096'
        '${Platform.pathSeparator}texture_00.png',
      ).exists(),
      isTrue,
    );
  });
}
