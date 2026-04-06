import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_client_for_appagent/vosk_model_locator.dart';

void main() {
  Future<void> createModelFiles(Directory dir) async {
    final requiredFiles = <String>[
      'am/final.mdl',
      'conf/mfcc.conf',
      'conf/model.conf',
      'graph/HCLr.fst',
      'graph/Gr.fst',
      'ivector/final.ie',
      'ivector/final.mat',
      'ivector/online_cmvn.conf',
    ];
    for (final relativePath in requiredFiles) {
      final file = File('${dir.path}${Platform.pathSeparator}$relativePath');
      await file.create(recursive: true);
      await file.writeAsString('ok');
    }
  }

  test('finds direct Vosk model root', () async {
    final tempDir = await Directory.systemTemp.createTemp('vosk-model-direct-');
    addTearDown(() async {
      if (await tempDir.exists()) {
        await tempDir.delete(recursive: true);
      }
    });

    await createModelFiles(tempDir);

    final modelRoot = await VoskModelLocator.findModelRoot(tempDir.path);

    expect(modelRoot, tempDir.path);
  });

  test('finds nested Vosk model root from extracted container', () async {
    final tempDir = await Directory.systemTemp.createTemp('vosk-model-nested-');
    addTearDown(() async {
      if (await tempDir.exists()) {
        await tempDir.delete(recursive: true);
      }
    });

    final nestedDir = Directory(
      '${tempDir.path}${Platform.pathSeparator}vosk-model-small-cn-0.22',
    );
    await nestedDir.create(recursive: true);
    await createModelFiles(nestedDir);

    final modelRoot = await VoskModelLocator.findModelRoot(tempDir.path);

    expect(modelRoot, nestedDir.path);
  });

  test('returns null when required files are missing', () async {
    final tempDir = await Directory.systemTemp.createTemp(
      'vosk-model-invalid-',
    );
    addTearDown(() async {
      if (await tempDir.exists()) {
        await tempDir.delete(recursive: true);
      }
    });

    await Directory(
      '${tempDir.path}${Platform.pathSeparator}graph',
    ).create(recursive: true);

    final modelRoot = await VoskModelLocator.findModelRoot(tempDir.path);

    expect(modelRoot, isNull);
  });

  test('rejects partial model when ivector directory is incomplete', () async {
    final tempDir = await Directory.systemTemp.createTemp(
      'vosk-model-invalid-ivector-',
    );
    addTearDown(() async {
      if (await tempDir.exists()) {
        await tempDir.delete(recursive: true);
      }
    });

    final baseRequiredFiles = <String>[
      'am/final.mdl',
      'conf/mfcc.conf',
      'conf/model.conf',
      'graph/HCLr.fst',
      'graph/Gr.fst',
    ];
    for (final relativePath in baseRequiredFiles) {
      final file = File('${tempDir.path}${Platform.pathSeparator}$relativePath');
      await file.create(recursive: true);
      await file.writeAsString('ok');
    }
    await Directory(
      '${tempDir.path}${Platform.pathSeparator}ivector',
    ).create(recursive: true);
    await File(
      '${tempDir.path}${Platform.pathSeparator}ivector${Platform.pathSeparator}final.ie',
    ).writeAsString('ok');

    final modelRoot = await VoskModelLocator.findModelRoot(tempDir.path);

    expect(modelRoot, isNull);
  });

  test('accepts official small-cn model layout without tree or phones txt', () async {
    final tempDir = await Directory.systemTemp.createTemp(
      'vosk-model-official-small-cn-',
    );
    addTearDown(() async {
      if (await tempDir.exists()) {
        await tempDir.delete(recursive: true);
      }
    });

    final extractedRoot = Directory(
      '${tempDir.path}${Platform.pathSeparator}vosk-model-small-cn-0.22',
    );
    await extractedRoot.create(recursive: true);
    await createModelFiles(extractedRoot);
    await File(
      '${extractedRoot.path}${Platform.pathSeparator}graph${Platform.pathSeparator}disambig_tid.int',
    ).writeAsString('ok');

    final modelRoot = await VoskModelLocator.findModelRoot(tempDir.path);

    expect(modelRoot, extractedRoot.path);
  });
}
