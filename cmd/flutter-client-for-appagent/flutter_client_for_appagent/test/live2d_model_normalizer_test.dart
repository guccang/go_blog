import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_client_for_appagent/main.dart'
    show Live2dModelLocator, Live2dModelNormalizer;

void main() {
  test(
    'normalizes bare VTube Studio model package for Cortana runtime',
    () async {
      final tempDir = await Directory.systemTemp.createTemp(
        'live2d-normalizer-',
      );
      addTearDown(() async {
        if (await tempDir.exists()) {
          await tempDir.delete(recursive: true);
        }
      });

      await _writeTinyPng(
        File(
          '${tempDir.path}${Platform.pathSeparator}test.4096'
          '${Platform.pathSeparator}texture_00.png',
        ),
        width: 4096,
        height: 4096,
      );
      await File(
        '${tempDir.path}${Platform.pathSeparator}test.moc3',
      ).writeAsBytes(<int>[1, 2, 3]);
      await File(
        '${tempDir.path}${Platform.pathSeparator}test.physics3.json',
      ).writeAsString('{}', encoding: utf8);
      await File(
        '${tempDir.path}${Platform.pathSeparator}happy.exp3.json',
      ).writeAsString('{}', encoding: utf8);
      final nestedExpressionFile = File(
        '${tempDir.path}${Platform.pathSeparator}expressions'
        '${Platform.pathSeparator}surprised.exp3.json',
      );
      await nestedExpressionFile.parent.create(recursive: true);
      await nestedExpressionFile.writeAsString('{}', encoding: utf8);
      await File(
        '${tempDir.path}${Platform.pathSeparator}wave.motion3.json',
      ).writeAsString('{}', encoding: utf8);
      final nestedMotionFile = File(
        '${tempDir.path}${Platform.pathSeparator}motions'
        '${Platform.pathSeparator}tap.motion3.json',
      );
      await nestedMotionFile.parent.create(recursive: true);
      await nestedMotionFile.writeAsString('{}', encoding: utf8);
      final modelFile = File(
        '${tempDir.path}${Platform.pathSeparator}test.model3.json',
      );
      await modelFile.writeAsString(
        jsonEncode(<String, dynamic>{
          'Version': 3,
          'FileReferences': <String, dynamic>{
            'Moc': 'test.moc3',
            'Textures': <String>['test.4096/texture_00.png'],
          },
          'Groups': <Map<String, dynamic>>[
            <String, dynamic>{
              'Target': 'Parameter',
              'Name': 'LipSync',
              'Ids': <String>[],
            },
          ],
        }),
        encoding: utf8,
      );

      final foundPath = await Live2dModelLocator.findModelJson(tempDir.path);
      expect(foundPath, modelFile.path);

      final result = await Live2dModelNormalizer.normalize(modelFile.path);

      expect(result.expressionCount, 2);
      expect(result.motionCount, 2);
      expect(result.textureMaxSize, 4096);
      expect(result.textureTotalPixels, 4096 * 4096);
      expect(await File(result.manifestPath).exists(), isTrue);
      final normalized =
          jsonDecode(await modelFile.readAsString(encoding: utf8))
              as Map<String, dynamic>;
      final refs = normalized['FileReferences'] as Map<String, dynamic>;
      expect(refs['Physics'], 'test.physics3.json');
      expect(refs['Expressions'], <dynamic>[
        <String, dynamic>{
          'Name': 'surprised',
          'File': 'expressions/surprised.exp3.json',
        },
        <String, dynamic>{'Name': 'happy', 'File': 'happy.exp3.json'},
      ]);
      expect(refs['Motions'], <String, dynamic>{
        'IdleWave': <dynamic>[
          <String, String>{'File': 'wave.motion3.json'},
        ],
        'Tap': <dynamic>[
          <String, String>{'File': 'motions/tap.motion3.json'},
        ],
      });
      final groups = normalized['Groups'] as List<dynamic>;
      expect(
        groups.any(
          (group) =>
              group is Map &&
              group['Name'] == 'LipSync' &&
              group['Ids'] is List &&
              (group['Ids'] as List).contains('ParamMouthOpenY'),
        ),
        isTrue,
      );
    },
  );

  test('rejects model package with missing texture file', () async {
    final tempDir = await Directory.systemTemp.createTemp(
      'live2d-normalizer-bad-',
    );
    addTearDown(() async {
      if (await tempDir.exists()) {
        await tempDir.delete(recursive: true);
      }
    });

    await File(
      '${tempDir.path}${Platform.pathSeparator}test.moc3',
    ).writeAsBytes(<int>[1, 2, 3]);
    final modelFile = File(
      '${tempDir.path}${Platform.pathSeparator}test.model3.json',
    );
    await modelFile.writeAsString(
      jsonEncode(<String, dynamic>{
        'Version': 3,
        'FileReferences': <String, dynamic>{
          'Moc': 'test.moc3',
          'Textures': <String>['missing/texture_00.png'],
        },
      }),
      encoding: utf8,
    );

    expect(
      () => Live2dModelNormalizer.normalize(modelFile.path),
      throwsA(
        isA<FormatException>().having(
          (error) => error.message,
          'message',
          contains('缺少贴图文件'),
        ),
      ),
    );
  });

  test(
    'reports clear error when oversized texture cannot be downscaled',
    () async {
      final tempDir = await Directory.systemTemp.createTemp(
        'live2d-normalizer-8192-',
      );
      addTearDown(() async {
        if (await tempDir.exists()) {
          await tempDir.delete(recursive: true);
        }
      });

      await _writeTinyPng(
        File(
          '${tempDir.path}${Platform.pathSeparator}test.8192'
          '${Platform.pathSeparator}texture_00.png',
        ),
        width: 8192,
        height: 8192,
      );
      await File(
        '${tempDir.path}${Platform.pathSeparator}test.moc3',
      ).writeAsBytes(<int>[1, 2, 3]);
      final modelFile = File(
        '${tempDir.path}${Platform.pathSeparator}test.model3.json',
      );
      await modelFile.writeAsString(
        jsonEncode(<String, dynamic>{
          'Version': 3,
          'FileReferences': <String, dynamic>{
            'Moc': 'test.moc3',
            'Textures': <String>['test.8192/texture_00.png'],
          },
        }),
        encoding: utf8,
      );

      expect(
        () => Live2dModelNormalizer.normalize(modelFile.path),
        throwsA(
          isA<FormatException>().having(
            (error) => error.message,
            'message',
            contains('贴图无法生成 WebView 兼容版'),
          ),
        ),
      );
    },
  );
}

Future<void> _writeTinyPng(
  File file, {
  required int width,
  required int height,
}) async {
  await file.parent.create(recursive: true);
  final bytes = <int>[
    137,
    80,
    78,
    71,
    13,
    10,
    26,
    10,
    0,
    0,
    0,
    13,
    73,
    72,
    68,
    82,
    ..._uint32(width),
    ..._uint32(height),
    8,
    6,
    0,
    0,
    0,
    0,
    0,
    0,
    0,
  ];
  await file.writeAsBytes(bytes);
}

List<int> _uint32(int value) {
  return <int>[
    (value >> 24) & 0xff,
    (value >> 16) & 0xff,
    (value >> 8) & 0xff,
    value & 0xff,
  ];
}
