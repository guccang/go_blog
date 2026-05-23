import 'dart:io';

class VoskModelLocator {
  static const List<String> _requiredRelativePaths = <String>[
    'am/final.mdl',
    'conf/mfcc.conf',
    'conf/model.conf',
    'graph/HCLr.fst',
    'graph/Gr.fst',
  ];
  static const List<String> _optionalIvectorPaths = <String>[
    'ivector/final.ie',
    'ivector/final.mat',
    'ivector/online_cmvn.conf',
  ];

  static Future<String?> findModelRoot(String rootPath) async {
    final trimmedPath = rootPath.trim();
    if (trimmedPath.isEmpty) {
      return null;
    }

    final rootDir = Directory(trimmedPath);
    if (!await rootDir.exists()) {
      return null;
    }

    final directRoot = await _matchModelRoot(rootDir);
    if (directRoot != null) {
      return directRoot;
    }

    try {
      await for (final entity in rootDir.list(followLinks: false)) {
        if (entity is! Directory) {
          continue;
        }
        final nestedRoot = await _matchModelRoot(entity);
        if (nestedRoot != null) {
          return nestedRoot;
        }
      }
    } catch (_) {
      return null;
    }

    return null;
  }

  static Future<bool> isModelRoot(String rootPath) async {
    final trimmedPath = rootPath.trim();
    if (trimmedPath.isEmpty) {
      return false;
    }
    return _hasRequiredFiles(Directory(trimmedPath));
  }

  static Future<String?> _matchModelRoot(Directory dir) async {
    if (await _hasRequiredFiles(dir)) {
      return dir.path;
    }
    return null;
  }

  static Future<bool> _hasRequiredFiles(Directory dir) async {
    for (final relativePath in _requiredRelativePaths) {
      final entityType = await FileSystemEntity.type(
        '${dir.path}${Platform.pathSeparator}$relativePath',
        followLinks: false,
      );
      if (entityType != FileSystemEntityType.file) {
        return false;
      }
    }
    final ivectorDirType = await FileSystemEntity.type(
      '${dir.path}${Platform.pathSeparator}ivector',
      followLinks: false,
    );
    if (ivectorDirType == FileSystemEntityType.directory) {
      for (final relativePath in _optionalIvectorPaths) {
        final entityType = await FileSystemEntity.type(
          '${dir.path}${Platform.pathSeparator}$relativePath',
          followLinks: false,
        );
        if (entityType != FileSystemEntityType.file) {
          return false;
        }
      }
    }
    return true;
  }
}
