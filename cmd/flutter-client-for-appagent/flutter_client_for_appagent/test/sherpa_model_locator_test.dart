import 'dart:convert';
import 'dart:io';

import 'package:flutter_client_for_appagent/sherpa_model_locator.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  Future<void> writeFile(Directory dir, String relativePath) async {
    final file = File('${dir.path}${Platform.pathSeparator}$relativePath');
    await file.parent.create(recursive: true);
    await file.writeAsString('ok', encoding: utf8);
  }

  Future<void> createAsrModel(Directory dir) async {
    await writeFile(dir, 'model.int8.onnx');
    await writeFile(dir, 'tokens.txt');
  }

  Future<void> createKwsModel(Directory dir, {bool int8 = true}) async {
    await writeFile(
      dir,
      int8
          ? 'encoder-epoch-12-avg-2-chunk-16-left-64.int8.onnx'
          : 'encoder-epoch-12-avg-2-chunk-16-left-64.onnx',
    );
    await writeFile(dir, 'decoder-epoch-12-avg-2-chunk-16-left-64.onnx');
    await writeFile(
      dir,
      int8
          ? 'joiner-epoch-12-avg-2-chunk-16-left-64.int8.onnx'
          : 'joiner-epoch-12-avg-2-chunk-16-left-64.onnx',
    );
    await writeFile(dir, 'tokens.txt');
  }

  test('finds direct SenseVoice ASR model root', () async {
    final tempDir = await Directory.systemTemp.createTemp('sherpa-asr-');
    addTearDown(() async {
      if (await tempDir.exists()) {
        await tempDir.delete(recursive: true);
      }
    });

    await createAsrModel(tempDir);

    final files = await SherpaModelLocator.findAsrModel(tempDir.path);

    expect(files, isNotNull);
    expect(files!.rootPath, tempDir.path);
    expect(files.modelPath, endsWith('model.int8.onnx'));
  });

  test('finds nested KWS model root from extracted archive', () async {
    final tempDir = await Directory.systemTemp.createTemp('sherpa-kws-');
    addTearDown(() async {
      if (await tempDir.exists()) {
        await tempDir.delete(recursive: true);
      }
    });

    final nestedDir = Directory(
      '${tempDir.path}${Platform.pathSeparator}sherpa-onnx-kws',
    );
    await createKwsModel(nestedDir, int8: false);

    final files = await SherpaModelLocator.findKwsModel(tempDir.path);

    expect(files, isNotNull);
    expect(files!.rootPath, nestedDir.path);
    expect(files.encoderPath, endsWith('.onnx'));
  });

  test('rejects incomplete Sherpa model directories', () async {
    final tempDir = await Directory.systemTemp.createTemp('sherpa-invalid-');
    addTearDown(() async {
      if (await tempDir.exists()) {
        await tempDir.delete(recursive: true);
      }
    });

    await writeFile(tempDir, 'tokens.txt');

    expect(
      await SherpaModelLocator.findModelRoot(tempDir.path, SherpaModelKind.asr),
      isNull,
    );
    expect(
      await SherpaModelLocator.findModelRoot(tempDir.path, SherpaModelKind.kws),
      isNull,
    );
  });
}
