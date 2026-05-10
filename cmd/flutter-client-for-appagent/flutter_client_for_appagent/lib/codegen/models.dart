class CodingProjectInfo {
  const CodingProjectInfo({
    required this.name,
    required this.agentId,
    required this.agent,
    required this.availableTools,
    required this.defaultTool,
    required this.claudeCodeSettings,
    required this.codexSettings,
    required this.defaultSettings,
  });

  final String name;
  final String agentId;
  final String agent;
  final List<String> availableTools;
  final String defaultTool;
  final List<String> claudeCodeSettings;
  final List<String> codexSettings;
  final String defaultSettings;

  String get qualifiedName => '$name@$agent';

  factory CodingProjectInfo.fromJson(Map<String, dynamic> json) {
    final availableTools =
        (json['available_tools'] as List<dynamic>? ?? const [])
            .map((item) => item.toString().trim().toLowerCase())
            .where((item) => item.isNotEmpty)
            .toList();
    final claudeCodeSettings =
        (json['claudecode_settings'] as List<dynamic>? ?? const [])
            .map((item) => item.toString().trim())
            .where((item) => item.isNotEmpty)
            .toList();
    final codexSettings = (json['codex_settings'] as List<dynamic>? ?? const [])
        .map((item) => item.toString().trim())
        .where((item) => item.isNotEmpty)
        .toList();
    return CodingProjectInfo(
      name: (json['name'] ?? '').toString().trim(),
      agentId: (json['agent_id'] ?? '').toString().trim(),
      agent: (json['agent'] ?? '').toString().trim(),
      availableTools: availableTools,
      defaultTool: (json['default_tool'] ?? '').toString().trim().toLowerCase(),
      claudeCodeSettings: claudeCodeSettings,
      codexSettings: codexSettings,
      defaultSettings: (json['default_settings'] ?? '').toString().trim(),
    );
  }
}

class DeployProjectInfo {
  const DeployProjectInfo({
    required this.name,
    required this.agentId,
    required this.agent,
    required this.deployTargets,
    required this.buildOnly,
  });

  final String name;
  final String agentId;
  final String agent;
  final List<String> deployTargets;
  final bool buildOnly;

  String get qualifiedName => '$name@$agent';

  factory DeployProjectInfo.fromJson(Map<String, dynamic> json) {
    final targets = (json['deploy_targets'] as List<dynamic>? ?? const [])
        .map((item) => item.toString().trim())
        .where((item) => item.isNotEmpty)
        .toList();
    return DeployProjectInfo(
      name: (json['name'] ?? '').toString().trim(),
      agentId: (json['agent_id'] ?? '').toString().trim(),
      agent: (json['agent'] ?? '').toString().trim(),
      deployTargets: targets,
      buildOnly: json['build_only'] == true,
    );
  }
}

class CodegenProjectsSnapshot {
  const CodegenProjectsSnapshot({
    required this.codingProjects,
    required this.deployProjects,
  });

  final List<CodingProjectInfo> codingProjects;
  final List<DeployProjectInfo> deployProjects;

  factory CodegenProjectsSnapshot.fromJson(Map<String, dynamic> json) {
    final codingProjects =
        (json['coding_projects'] as List<dynamic>? ?? const [])
            .map(
              (item) =>
                  CodingProjectInfo.fromJson(item as Map<String, dynamic>),
            )
            .where((item) => item.name.isNotEmpty && item.agent.isNotEmpty)
            .toList();
    final deployProjects =
        (json['deploy_projects'] as List<dynamic>? ?? const [])
            .map(
              (item) =>
                  DeployProjectInfo.fromJson(item as Map<String, dynamic>),
            )
            .where((item) => item.name.isNotEmpty && item.agent.isNotEmpty)
            .toList();
    return CodegenProjectsSnapshot(
      codingProjects: codingProjects,
      deployProjects: deployProjects,
    );
  }
}

enum CodegenLaunchMode { code, deploy, backup }

enum CodegenHistoryBackupType { full, incremental }

extension CodegenHistoryBackupTypeLabel on CodegenHistoryBackupType {
  String get label =>
      this == CodegenHistoryBackupType.full ? '全量备份' : '增量备份';
}

class CodegenHistoryBackupItem {
  const CodegenHistoryBackupItem({
    required this.backupType,
    required this.fileName,
    required this.fileSize,
    required this.objectKey,
    required this.createdAt,
  });

  final String backupType;
  final String fileName;
  final int fileSize;
  final String objectKey;
  final DateTime createdAt;

  String get label {
    final type = backupType == CodegenHistoryBackupType.full.name
        ? '全量备份'
        : '增量备份';
    return '$type ${createdAt.toLocal()}';
  }

  factory CodegenHistoryBackupItem.fromJson(Map<String, dynamic> json) {
    final createdAtValue = json['created_at'];
    final createdAtMillis = createdAtValue is int
        ? createdAtValue
        : int.tryParse('$createdAtValue') ?? 0;
    return CodegenHistoryBackupItem(
      backupType: (json['backup_type'] ?? '').toString().trim(),
      fileName: (json['file_name'] ?? '').toString().trim(),
      fileSize: int.tryParse('${json['file_size'] ?? 0}') ?? 0,
      objectKey: (json['object_key'] ?? '').toString().trim(),
      createdAt: createdAtMillis > 0
          ? DateTime.fromMillisecondsSinceEpoch(createdAtMillis)
          : DateTime.fromMillisecondsSinceEpoch(0),
    );
  }
}

class CodegenProcessEntry {
  const CodegenProcessEntry({
    required this.timestamp,
    required this.content,
    required this.origin,
    this.sessionId = '',
  });

  final DateTime timestamp;
  final String content;
  final String origin;
  final String sessionId;

  Map<String, dynamic> toJson() {
    return {
      'timestamp': timestamp.toIso8601String(),
      'content': content,
      'origin': origin,
      'session_id': sessionId,
    };
  }

  factory CodegenProcessEntry.fromJson(Map<String, dynamic> json) {
    return CodegenProcessEntry(
      timestamp: DateTime.parse(json['timestamp'] as String),
      content: (json['content'] ?? '').toString(),
      origin: (json['origin'] ?? '').toString(),
      sessionId: (json['session_id'] ?? '').toString(),
    );
  }
}

class CodegenHistoryItem {
  const CodegenHistoryItem({
    required this.id,
    required this.timestamp,
    required this.command,
    required this.mode,
    this.locked = false,
    this.completed = false,
    this.requestId = '',
    this.processEntries = const <CodegenProcessEntry>[],
  });

  final String id;
  final DateTime timestamp;
  final String command;
  final CodegenLaunchMode mode;
  final bool locked;
  final bool completed;
  final String requestId;
  final List<CodegenProcessEntry> processEntries;

  CodegenHistoryItem copyWith({
    bool? locked,
    bool? completed,
    String? requestId,
    List<CodegenProcessEntry>? processEntries,
  }) {
    return CodegenHistoryItem(
      id: id,
      timestamp: timestamp,
      command: command,
      mode: mode,
      locked: locked ?? this.locked,
      completed: completed ?? this.completed,
      requestId: requestId ?? this.requestId,
      processEntries: processEntries ?? this.processEntries,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'timestamp': timestamp.toIso8601String(),
      'command': command,
      'mode': mode.name,
      'locked': locked,
      'completed': completed,
      'request_id': requestId,
      'process_entries': processEntries.map((item) => item.toJson()).toList(),
    };
  }

  factory CodegenHistoryItem.fromJson(Map<String, dynamic> json) {
    return CodegenHistoryItem(
      id: (json['id'] ?? '').toString(),
      timestamp: DateTime.parse(json['timestamp'] as String),
      command: (json['command'] ?? '').toString(),
      mode: _parseLaunchMode((json['mode'] ?? '').toString()),
      locked: json['locked'] == true,
      completed: json['completed'] == true,
      requestId: (json['request_id'] ?? '').toString(),
      processEntries: (json['process_entries'] as List<dynamic>? ?? const [])
          .map(
            (item) =>
                CodegenProcessEntry.fromJson(item as Map<String, dynamic>),
          )
          .toList(),
    );
  }

  static CodegenLaunchMode _parseLaunchMode(String value) {
    switch (value.trim().toLowerCase()) {
      case 'deploy':
        return CodegenLaunchMode.deploy;
      case 'backup':
        return CodegenLaunchMode.backup;
      case 'code':
      default:
        return CodegenLaunchMode.code;
    }
  }
}

class CodegenHistoryCommandDetails {
  const CodegenHistoryCommandDetails({
    required this.mode,
    required this.projectQualifiedName,
    required this.requestText,
    required this.tool,
    required this.claudeSettings,
    required this.target,
    required this.extraArgs,
    required this.autoDeploy,
    required this.packOnly,
  });

  final CodegenLaunchMode mode;
  final String projectQualifiedName;
  final String requestText;
  final String tool;
  final String claudeSettings;
  final String target;
  final String extraArgs;
  final bool autoDeploy;
  final bool packOnly;

  factory CodegenHistoryCommandDetails.parse(CodegenHistoryItem item) {
    final tokens = item.command
        .trim()
        .split(RegExp(r'\s+'))
        .where((token) => token.isNotEmpty)
        .toList();
    if (tokens.length < 3 || tokens.first != '/cg') {
      return CodegenHistoryCommandDetails(
        mode: item.mode,
        projectQualifiedName: '',
        requestText: '',
        tool: '',
        claudeSettings: '',
        target: '',
        extraArgs: '',
        autoDeploy: false,
        packOnly: false,
      );
    }

    final action = tokens[1];
    if (action == 'history-backup' || action == 'backup-history') {
      var backupType = '';
      for (var i = 2; i < tokens.length; i++) {
        if (tokens[i] == '--type' && i + 1 < tokens.length) {
          backupType = tokens[i + 1];
          break;
        }
      }
      return CodegenHistoryCommandDetails(
        mode: CodegenLaunchMode.backup,
        projectQualifiedName: '',
        requestText: backupType,
        tool: '',
        claudeSettings: '',
        target: '',
        extraArgs: '',
        autoDeploy: false,
        packOnly: false,
      );
    }

    if (action == 'start' || action == 'debug') {
      var autoDeploy = false;
      var tool = '';
      var claudeSettings = '';
      final projectQualifiedName = tokens[2];
      var requestStart = 3;
      while (requestStart < tokens.length) {
        final token = tokens[requestStart];
        if (token == '!deploy') {
          autoDeploy = true;
          requestStart++;
          continue;
        }
        if (token.startsWith('@')) {
          tool = token.substring(1);
          requestStart++;
          continue;
        }
        if (token == '--settings' && requestStart + 1 < tokens.length) {
          claudeSettings = tokens[requestStart + 1];
          requestStart += 2;
          continue;
        }
        break;
      }
      final requestText = requestStart < tokens.length
          ? tokens.sublist(requestStart).join(' ')
          : '';
      return CodegenHistoryCommandDetails(
        mode: CodegenLaunchMode.code,
        projectQualifiedName: projectQualifiedName,
        requestText: requestText,
        tool: tool,
        claudeSettings: claudeSettings,
        target: '',
        extraArgs: '',
        autoDeploy: autoDeploy,
        packOnly: false,
      );
    }

    if (action == 'deploy') {
      var target = '';
      var packOnly = false;
      final projectQualifiedName = tokens[2];
      final args = <String>[];
      for (final token in tokens.skip(3)) {
        if (token.startsWith('#') && target.isEmpty) {
          target = token.substring(1);
          continue;
        }
        if (token == '!pack') {
          packOnly = true;
          continue;
        }
        args.add(token);
      }
      return CodegenHistoryCommandDetails(
        mode: CodegenLaunchMode.deploy,
        projectQualifiedName: projectQualifiedName,
        requestText: '',
        tool: '',
        claudeSettings: '',
        target: target,
        extraArgs: args.join(' '),
        autoDeploy: false,
        packOnly: packOnly,
      );
    }

    return CodegenHistoryCommandDetails(
      mode: item.mode,
      projectQualifiedName: '',
      requestText: '',
      tool: '',
      claudeSettings: '',
      target: '',
      extraArgs: '',
      autoDeploy: false,
      packOnly: false,
    );
  }
}
