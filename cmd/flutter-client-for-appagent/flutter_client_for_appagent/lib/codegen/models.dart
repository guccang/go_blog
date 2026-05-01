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

enum CodegenLaunchMode { code, deploy }

class CodegenHistoryItem {
  const CodegenHistoryItem({
    required this.timestamp,
    required this.command,
    required this.mode,
    this.locked = false,
  });

  final DateTime timestamp;
  final String command;
  final CodegenLaunchMode mode;
  final bool locked;

  CodegenHistoryItem copyWith({bool? locked}) {
    return CodegenHistoryItem(
      timestamp: timestamp,
      command: command,
      mode: mode,
      locked: locked ?? this.locked,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'timestamp': timestamp.toIso8601String(),
      'command': command,
      'mode': mode.name,
      'locked': locked,
    };
  }

  factory CodegenHistoryItem.fromJson(Map<String, dynamic> json) {
    return CodegenHistoryItem(
      timestamp: DateTime.parse(json['timestamp'] as String),
      command: (json['command'] ?? '').toString(),
      mode: json['mode'] == 'deploy'
          ? CodegenLaunchMode.deploy
          : CodegenLaunchMode.code,
      locked: json['locked'] == true,
    );
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
    if (action == 'start') {
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
