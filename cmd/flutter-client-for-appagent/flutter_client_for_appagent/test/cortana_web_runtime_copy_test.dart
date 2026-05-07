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

  test('adds sibling expressions and motions to bare model json', () async {
    final tempDir = await Directory.systemTemp.createTemp(
      'cortana-web-runtime-model-',
    );
    addTearDown(() async {
      if (await tempDir.exists()) {
        await tempDir.delete(recursive: true);
      }
    });

    final modelFile = File(
      '${tempDir.path}${Platform.pathSeparator}baimeimo.model3.json',
    );
    await modelFile.writeAsString(
      jsonEncode(<String, dynamic>{
        'Version': 3,
        'FileReferences': <String, dynamic>{
          'Moc': 'baimeimo.moc3',
          'Textures': <String>['baimeimo.4096/texture_00.png'],
          'Expressions': <Map<String, String>>[
            <String, String>{'Name': 'existing', 'File': 'existing.exp3.json'},
          ],
        },
      }),
      encoding: utf8,
    );
    await File(
      '${tempDir.path}${Platform.pathSeparator}existing.exp3.json',
    ).writeAsString('{}', encoding: utf8);
    await File(
      '${tempDir.path}${Platform.pathSeparator}bai.exp3.json',
    ).writeAsString('{}', encoding: utf8);
    final nestedMotionFile = File(
      '${tempDir.path}${Platform.pathSeparator}motions'
      '${Platform.pathSeparator}shanziguan.motion3.json',
    );
    await nestedMotionFile.parent.create(recursive: true);
    await nestedMotionFile.writeAsString('{}', encoding: utf8);

    final changed = await addMissingCortanaModelReferencesForWebRuntime(
      modelFile,
    );

    expect(changed, isTrue);
    final decoded =
        jsonDecode(await modelFile.readAsString(encoding: utf8))
            as Map<String, dynamic>;
    final refs = decoded['FileReferences'] as Map<String, dynamic>;
    expect(refs['Expressions'], <dynamic>[
      <String, dynamic>{'Name': 'existing', 'File': 'existing.exp3.json'},
      <String, dynamic>{'Name': 'bai', 'File': 'bai.exp3.json'},
    ]);
    expect(refs['Motions'], <String, dynamic>{
      'Idle': <dynamic>[
        <String, String>{'File': 'motions/shanziguan.motion3.json'},
      ],
    });
  });

  test('slows Live2D motion timeline for web runtime', () async {
    final tempDir = await Directory.systemTemp.createTemp(
      'cortana-web-runtime-motion-speed-',
    );
    addTearDown(() async {
      if (await tempDir.exists()) {
        await tempDir.delete(recursive: true);
      }
    });

    final motionFile = File(
      '${tempDir.path}${Platform.pathSeparator}motions'
      '${Platform.pathSeparator}wave.motion3.json',
    );
    await motionFile.parent.create(recursive: true);
    await motionFile.writeAsString(
      jsonEncode(<String, dynamic>{
        'Version': 3,
        'Meta': <String, dynamic>{'Duration': 2.0},
        'Curves': <Map<String, dynamic>>[
          <String, dynamic>{
            'Target': 'Parameter',
            'Id': 'ParamArmLA',
            'Segments': <num>[0, 0, 0, 1, 1, 1, 1.2, 0.2, 1.6, 0.8, 2, 0],
          },
        ],
        'Events': <Map<String, dynamic>>[
          <String, dynamic>{'Time': 1.5, 'Value': 'event'},
        ],
      }),
      encoding: utf8,
    );

    final changed = await slowLive2dMotionFilesForWebRuntime(
      tempDir,
      playbackSpeed: 0.5,
    );

    expect(changed, 1);
    final decoded =
        jsonDecode(await motionFile.readAsString(encoding: utf8))
            as Map<String, dynamic>;
    final meta = decoded['Meta'] as Map<String, dynamic>;
    expect(meta['Duration'], 4.0);
    expect(meta['CortanaPlaybackSpeed'], '0.50');
    final curves = decoded['Curves'] as List<dynamic>;
    final curve = curves.single as Map<String, dynamic>;
    expect(curve['Segments'], <dynamic>[
      0.0,
      0,
      0,
      2.0,
      1,
      1,
      2.4,
      0.2,
      3.2,
      0.8,
      4.0,
      0,
    ]);
    final events = decoded['Events'] as List<dynamic>;
    expect((events.single as Map<String, dynamic>)['Time'], 3.0);

    final changedAgain = await slowLive2dMotionFilesForWebRuntime(
      tempDir,
      playbackSpeed: 0.5,
    );
    expect(changedAgain, 0);
  });
}
