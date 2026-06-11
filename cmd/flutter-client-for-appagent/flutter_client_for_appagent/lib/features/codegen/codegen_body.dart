import 'package:flutter/material.dart';

import 'models.dart';

class CodegenBodyPalette {
  const CodegenBodyPalette({
    required this.backgroundTop,
    required this.surfaceMuted,
    required this.backgroundBottom,
    required this.surface,
    required this.border,
    required this.textPrimary,
    required this.textSecondary,
    required this.error,
  });

  final Color backgroundTop;
  final Color surfaceMuted;
  final Color backgroundBottom;
  final Color surface;
  final Color border;
  final Color textPrimary;
  final Color textSecondary;
  final Color error;
}

class CodegenBody extends StatelessWidget {
  const CodegenBody({
    super.key,
    required this.palette,
    required this.loading,
    required this.errorText,
    required this.mode,
    required this.codingProjects,
    required this.deployProjects,
    required this.selectedCodingProject,
    required this.selectedDeployProject,
    required this.selectedCodeTool,
    required this.selectedToolSettings,
    required this.resumeLastSession,
    required this.resumeLastSessionEnabled,
    required this.selectedCodeToolOptions,
    required this.selectedToolSettingsOptions,
    required this.selectedDeployTarget,
    required this.commandPreview,
    required this.autoDeploy,
    required this.deployPackOnly,
    required this.debugBundleMode,
    required this.runningCodeCount,
    required this.runningDeployCount,
    required this.codegenPromptController,
    required this.codegenCodeSearchController,
    required this.codegenDeploySearchController,
    required this.deployArgsController,
    required this.history,
    required this.onRefresh,
    required this.onModeChanged,
    required this.onSearchChanged,
    required this.onCodeProjectChanged,
    required this.onCodeToolChanged,
    required this.onToolSettingsChanged,
    required this.onResumeLastSessionChanged,
    required this.onPromptChanged,
    required this.onAutoDeployChanged,
    required this.onDebugBundleModeChanged,
    required this.onDeployProjectChanged,
    required this.onDeployTargetChanged,
    required this.onDeployPackOnlyChanged,
    required this.onDeployArgsChanged,
    required this.onSend,
    required this.onCommitAndPush,
    required this.onSessionStatus,
    required this.onSessionStop,
    required this.onBackupHistory,
    required this.onLoadHistoryBackup,
    required this.onClearHistory,
    required this.onShowHistoryDetails,
    required this.onReExecute,
    required this.onToggleLock,
    required this.sending,
  });

  final CodegenBodyPalette palette;
  final bool loading;
  final bool sending;
  final String errorText;
  final CodegenLaunchMode mode;
  final List<CodingProjectInfo> codingProjects;
  final List<DeployProjectInfo> deployProjects;
  final CodingProjectInfo? selectedCodingProject;
  final DeployProjectInfo? selectedDeployProject;
  final String selectedCodeTool;
  final String selectedToolSettings;
  final bool resumeLastSession;
  final bool resumeLastSessionEnabled;
  final List<String> selectedCodeToolOptions;
  final List<String> selectedToolSettingsOptions;
  final String selectedDeployTarget;
  final String commandPreview;
  final bool autoDeploy;
  final bool deployPackOnly;
  final bool debugBundleMode;
  final int runningCodeCount;
  final int runningDeployCount;
  final TextEditingController codegenPromptController;
  final TextEditingController codegenCodeSearchController;
  final TextEditingController codegenDeploySearchController;
  final TextEditingController deployArgsController;
  final List<CodegenHistoryItem> history;
  final VoidCallback onRefresh;
  final ValueChanged<CodegenLaunchMode> onModeChanged;
  final ValueChanged<String> onSearchChanged;
  final ValueChanged<String?> onCodeProjectChanged;
  final ValueChanged<String?> onCodeToolChanged;
  final ValueChanged<String?> onToolSettingsChanged;
  final ValueChanged<bool> onResumeLastSessionChanged;
  final ValueChanged<String> onPromptChanged;
  final ValueChanged<bool> onAutoDeployChanged;
  final ValueChanged<bool> onDebugBundleModeChanged;
  final ValueChanged<String?> onDeployProjectChanged;
  final ValueChanged<String?> onDeployTargetChanged;
  final ValueChanged<bool> onDeployPackOnlyChanged;
  final ValueChanged<String> onDeployArgsChanged;
  final VoidCallback onSend;
  final VoidCallback onCommitAndPush;
  final VoidCallback onSessionStatus;
  final VoidCallback onSessionStop;
  final ValueChanged<CodegenHistoryBackupType> onBackupHistory;
  final VoidCallback onLoadHistoryBackup;
  final VoidCallback onClearHistory;
  final ValueChanged<CodegenHistoryItem> onShowHistoryDetails;
  final ValueChanged<CodegenHistoryItem> onReExecute;
  final ValueChanged<CodegenHistoryItem> onToggleLock;

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: BoxDecoration(
        gradient: LinearGradient(
          colors: <Color>[
            palette.backgroundTop,
            palette.surfaceMuted,
            palette.backgroundBottom,
          ],
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
        ),
      ),
      child: SafeArea(
        child: ListView(
          padding: const EdgeInsets.fromLTRB(10, 10, 10, 18),
          children: [
            _buildFormCard(context),
            const SizedBox(height: 16),
            if (history.isNotEmpty) _buildHistoryCard(context),
          ],
        ),
      ),
    );
  }

  Widget _buildFormCard(BuildContext context) {
    return Container(
      padding: const EdgeInsets.fromLTRB(16, 16, 16, 16),
      decoration: BoxDecoration(
        color: palette.surface.withValues(alpha: 0.96),
        borderRadius: BorderRadius.circular(24),
        border: Border.all(color: palette.border),
        boxShadow: [
          BoxShadow(
            blurRadius: 18,
            color: Colors.black.withValues(alpha: 0.18),
            offset: const Offset(0, 10),
          ),
        ],
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(
                  '编码发布',
                  style: TextStyle(
                    color: palette.textPrimary,
                    fontSize: 20,
                    fontWeight: FontWeight.w800,
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 8),
          Text(
            '从 acp-agent 和 deploy-agent 的现有项目中快速选择，直接发送 /cg 命令。',
            style: TextStyle(color: palette.textSecondary, height: 1.4),
          ),
          const SizedBox(height: 14),
          Align(
            alignment: Alignment.centerLeft,
            child: OutlinedButton.icon(
              onPressed: loading ? null : onRefresh,
              icon: loading
                  ? const SizedBox(
                      width: 16,
                      height: 16,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : const Icon(Icons.refresh_rounded),
              label: Text(loading ? '刷新中...' : '刷新编码和发布项目'),
            ),
          ),
          const SizedBox(height: 14),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              ChoiceChip(
                selected: mode == CodegenLaunchMode.code,
                label: Text(
                  runningCodeCount > 0 ? '编码 $runningCodeCount' : '编码',
                ),
                onSelected: (_) => onModeChanged(CodegenLaunchMode.code),
              ),
              ChoiceChip(
                selected: mode == CodegenLaunchMode.deploy,
                label: Text(
                  runningDeployCount > 0 ? '发布 $runningDeployCount' : '发布',
                ),
                onSelected: (_) => onModeChanged(CodegenLaunchMode.deploy),
              ),
            ],
          ),
          const SizedBox(height: 14),
          TextField(
            controller: mode == CodegenLaunchMode.code
                ? codegenCodeSearchController
                : codegenDeploySearchController,
            onChanged: onSearchChanged,
            decoration: const InputDecoration(
              labelText: '搜索项目',
              hintText: '支持项目名、agent、多关键词模糊搜索',
              prefixIcon: Icon(Icons.search_rounded),
              isDense: true,
            ),
          ),
          const SizedBox(height: 14),
          SizedBox(
            height: 3,
            child: AnimatedOpacity(
              opacity: loading ? 1 : 0,
              duration: const Duration(milliseconds: 160),
              child: const LinearProgressIndicator(minHeight: 3),
            ),
          ),
          if (errorText.isNotEmpty) ...[
            const SizedBox(height: 12),
            Text(
              errorText,
              style: TextStyle(
                color: palette.error,
                fontWeight: FontWeight.w600,
              ),
            ),
          ],
          const SizedBox(height: 14),
          if (mode == CodegenLaunchMode.code)
            _buildCodeModeFields()
          else
            _buildDeployModeFields(),
          const SizedBox(height: 10),
          Text(
            '命令预览',
            style: TextStyle(
              color: palette.textSecondary,
              fontWeight: FontWeight.w700,
            ),
          ),
          const SizedBox(height: 8),
          SelectableText(
            commandPreview,
            style: TextStyle(
              color: palette.textPrimary,
              fontFamily: 'monospace',
              height: 1.5,
            ),
          ),
          const SizedBox(height: 16),
          LayoutBuilder(
            builder: (context, constraints) {
              final stacked = constraints.maxWidth < 420;
              final sendButton = FilledButton.icon(
                onPressed: sending || loading ? null : onSend,
                icon: Icon(
                  mode == CodegenLaunchMode.code
                      ? Icons.terminal_rounded
                      : Icons.rocket_launch_rounded,
                ),
                label: Text(
                  mode == CodegenLaunchMode.code ? '发送编码命令' : '发送发布命令',
                ),
              );
              final commitButton = OutlinedButton.icon(
                onPressed: sending || loading ? null : onCommitAndPush,
                icon: const Icon(Icons.account_tree_rounded),
                label: const Text('git提交'),
              );
              final statusButton = OutlinedButton.icon(
                onPressed: sending || loading ? null : onSessionStatus,
                icon: const Icon(Icons.timelapse_rounded),
                label: const Text('进度'),
              );
              final stopButton = OutlinedButton.icon(
                onPressed: sending || loading ? null : onSessionStop,
                style: OutlinedButton.styleFrom(
                  foregroundColor: palette.error,
                ),
                icon: const Icon(Icons.stop_circle_outlined),
                label: const Text('停止'),
              );
              if (stacked) {
                return Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    sendButton,
                    const SizedBox(height: 8),
                    commitButton,
                    const SizedBox(height: 8),
                    Row(
                      children: [
                        Expanded(child: statusButton),
                        const SizedBox(width: 8),
                        Expanded(child: stopButton),
                      ],
                    ),
                  ],
                );
              }
              return Row(
                children: [
                  Expanded(child: sendButton),
                  const SizedBox(width: 8),
                  commitButton,
                  const SizedBox(width: 8),
                  statusButton,
                  const SizedBox(width: 8),
                  stopButton,
                ],
              );
            },
          ),
        ],
      ),
    );
  }

  Widget _buildCodeModeFields() {
    final resumeIdentifier = codegenResumeIdentifierForTool(selectedCodeTool);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        DropdownButtonFormField<String>(
          initialValue: _resolveSelectedValue(
            codingProjects.map((item) => item.qualifiedName).toList(),
            selectedCodingProject?.qualifiedName,
          ),
          items: codingProjects
              .map(
                (project) => DropdownMenuItem<String>(
                  value: project.qualifiedName,
                  child: Text(project.qualifiedName),
                ),
              )
              .toList(),
          onChanged: codingProjects.isEmpty ? null : onCodeProjectChanged,
          decoration: const InputDecoration(labelText: '编码项目', isDense: true),
        ),
        const SizedBox(height: 12),
        DropdownButtonFormField<String>(
          initialValue: _resolveSelectedValue(
            selectedCodeToolOptions,
            selectedCodeTool,
          ),
          items: selectedCodeToolOptions
              .map(
                (tool) =>
                    DropdownMenuItem<String>(value: tool, child: Text(tool)),
              )
              .toList(),
          onChanged: selectedCodeToolOptions.isEmpty ? null : onCodeToolChanged,
          decoration: const InputDecoration(labelText: '编码工具', isDense: true),
        ),
        if (selectedToolSettingsOptions.isNotEmpty) ...[
          const SizedBox(height: 12),
          DropdownButtonFormField<String>(
            initialValue: _resolveSelectedValue(
              selectedToolSettingsOptions,
              selectedToolSettings,
            ),
            items: selectedToolSettingsOptions
                .map(
                  (settings) => DropdownMenuItem<String>(
                    value: settings,
                    child: Text(settings),
                  ),
                )
                .toList(),
            onChanged: onToolSettingsChanged,
            decoration: const InputDecoration(
              labelText: '编码工具配置',
              helperText: '配置来自当前选中编码工具的 settings',
              isDense: true,
            ),
          ),
        ],
        const SizedBox(height: 12),
        TextField(
          controller: codegenPromptController,
          onChanged: onPromptChanged,
          minLines: 4,
          maxLines: 7,
          decoration: const InputDecoration(
            labelText: '编码需求',
            hintText: '例如：增加一个页签可以快速选择项目并执行编码发布',
            alignLabelWithHint: true,
          ),
        ),
        const SizedBox(height: 12),
        SwitchListTile(
          value: resumeLastSession,
          onChanged: resumeLastSessionEnabled
              ? onResumeLastSessionChanged
              : null,
          contentPadding: EdgeInsets.zero,
          title: const Text('继续上次会话'),
          subtitle: Text(
            resumeIdentifier.isEmpty
                ? '当前工具不支持继续上次会话'
                : '启用后将使用继续标识符：$resumeIdentifier',
          ),
        ),
        const SizedBox(height: 4),
        SwitchListTile(
          value: autoDeploy,
          onChanged: debugBundleMode ? null : onAutoDeployChanged,
          contentPadding: EdgeInsets.zero,
          title: const Text('编码完成后自动部署'),
        ),
        SwitchListTile(
          value: debugBundleMode,
          onChanged: onDebugBundleModeChanged,
          contentPadding: EdgeInsets.zero,
          title: const Text('携带 Debug Bundle'),
          subtitle: const Text(
            '发送前收集 Flutter 客户端和 agent 日志，交给 ACP debug 会话定位问题',
          ),
        ),
      ],
    );
  }

  Widget _buildDeployModeFields() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        DropdownButtonFormField<String>(
          initialValue: _resolveSelectedValue(
            deployProjects.map((item) => item.qualifiedName).toList(),
            selectedDeployProject?.qualifiedName,
          ),
          items: deployProjects
              .map(
                (project) => DropdownMenuItem<String>(
                  value: project.qualifiedName,
                  child: Text(project.qualifiedName),
                ),
              )
              .toList(),
          onChanged: deployProjects.isEmpty ? null : onDeployProjectChanged,
          decoration: const InputDecoration(labelText: '部署项目', isDense: true),
        ),
        const SizedBox(height: 12),
        DropdownButtonFormField<String>(
          initialValue: _resolveSelectedValue(
            selectedDeployProject?.deployTargets ?? const <String>[],
            selectedDeployTarget,
          ),
          items: (selectedDeployProject?.deployTargets ?? const <String>[])
              .map(
                (target) => DropdownMenuItem<String>(
                  value: target,
                  child: Text(target),
                ),
              )
              .toList(),
          onChanged:
              selectedDeployProject == null ||
                  selectedDeployProject!.deployTargets.isEmpty
              ? null
              : onDeployTargetChanged,
          decoration: const InputDecoration(
            labelText: '部署目标',
            hintText: '可选',
            isDense: true,
          ),
        ),
        const SizedBox(height: 12),
        SwitchListTile(
          value: deployPackOnly,
          onChanged: onDeployPackOnlyChanged,
          contentPadding: EdgeInsets.zero,
          title: const Text('仅打包'),
          subtitle: selectedDeployProject?.buildOnly == true
              ? const Text('该项目仅支持打包，不能直接部署')
              : null,
        ),
        const SizedBox(height: 12),
        TextField(
          controller: deployArgsController,
          onChanged: onDeployArgsChanged,
          decoration: const InputDecoration(
            labelText: '发布参数',
            hintText: '例如：--version 3.2.2 --desc 灰度体验版',
            helperText: '会原样追加到 /cg deploy 命令末尾',
            isDense: true,
          ),
        ),
      ],
    );
  }

  Widget _buildHistoryCard(BuildContext context) {
    return Container(
      padding: const EdgeInsets.fromLTRB(16, 16, 16, 16),
      decoration: BoxDecoration(
        color: palette.surface.withValues(alpha: 0.96),
        borderRadius: BorderRadius.circular(24),
        border: Border.all(color: palette.border),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Text.rich(
                  TextSpan(
                    text: '最近任务',
                    style: TextStyle(
                      color: palette.textPrimary,
                      fontSize: 18,
                      fontWeight: FontWeight.w800,
                    ),
                    children: [
                      TextSpan(
                        text:
                            ' (编码${history.where((e) => e.mode == CodegenLaunchMode.code).length}条, '
                            '发布${history.where((e) => e.mode == CodegenLaunchMode.deploy).length}条, '
                            '备份${history.where((e) => e.mode == CodegenLaunchMode.backup).length}条)',
                        style: TextStyle(
                          color: palette.textSecondary,
                          fontSize: 14,
                          fontWeight: FontWeight.w500,
                        ),
                      ),
                    ],
                  ),
                ),
              ),
              TextButton.icon(
                onPressed: onClearHistory,
                icon: const Icon(Icons.delete_sweep_rounded),
                label: const Text('清空'),
              ),
            ],
          ),
          const SizedBox(height: 8),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              OutlinedButton.icon(
                onPressed: () =>
                    onBackupHistory(CodegenHistoryBackupType.incremental),
                icon: const Icon(Icons.backup_table_rounded),
                label: const Text('增量备份'),
              ),
              OutlinedButton.icon(
                onPressed: () => onBackupHistory(CodegenHistoryBackupType.full),
                icon: const Icon(Icons.cloud_upload_rounded),
                label: const Text('全量备份'),
              ),
              OutlinedButton.icon(
                onPressed: onLoadHistoryBackup,
                icon: const Icon(Icons.cloud_download_rounded),
                label: const Text('加载备份'),
              ),
            ],
          ),
          const SizedBox(height: 10),
          ListView.separated(
            shrinkWrap: true,
            physics: const NeverScrollableScrollPhysics(),
            itemCount: history.length,
            separatorBuilder: (context, index) => const SizedBox(height: 10),
            itemBuilder: (context, index) {
              final item = history[index];
              return Material(
                color: Colors.transparent,
                child: InkWell(
                  borderRadius: BorderRadius.circular(16),
                  onTap: () => onShowHistoryDetails(item),
                  child: Container(
                    padding: const EdgeInsets.fromLTRB(12, 12, 12, 12),
                    decoration: BoxDecoration(
                      color: palette.surfaceMuted,
                      borderRadius: BorderRadius.circular(16),
                      border: Border.all(color: palette.border),
                    ),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Row(
                          children: [
                            Container(
                              padding: const EdgeInsets.symmetric(
                                horizontal: 8,
                                vertical: 4,
                              ),
                              decoration: BoxDecoration(
                                color: _modeColor(
                                  item.mode,
                                ).withValues(alpha: 0.16),
                                borderRadius: BorderRadius.circular(999),
                              ),
                              child: Text(
                                _modeLabel(item.mode),
                                style: TextStyle(
                                  color: _modeColor(item.mode),
                                  fontSize: 12,
                                  fontWeight: FontWeight.w700,
                                ),
                              ),
                            ),
                            if (item.locked) ...[
                              const SizedBox(width: 6),
                              Icon(
                                Icons.lock_rounded,
                                size: 14,
                                color: palette.textSecondary,
                              ),
                            ],
                            const Spacer(),
                            Text(
                              _formatTime(item.timestamp),
                              style: TextStyle(
                                color: palette.textSecondary,
                                fontSize: 12,
                              ),
                            ),
                          ],
                        ),
                        const SizedBox(height: 10),
                        Text(
                          item.command,
                          maxLines: 3,
                          overflow: TextOverflow.ellipsis,
                          style: TextStyle(
                            color: palette.textPrimary,
                            height: 1.45,
                            fontFamily: 'monospace',
                          ),
                        ),
                        const SizedBox(height: 8),
                        Row(
                          children: [
                            Container(
                              padding: const EdgeInsets.symmetric(
                                horizontal: 8,
                                vertical: 3,
                              ),
                              decoration: BoxDecoration(
                                color: item.completed
                                    ? Colors.green.withValues(alpha: 0.15)
                                    : Colors.orange.withValues(alpha: 0.16),
                                borderRadius: BorderRadius.circular(999),
                              ),
                              child: Text(
                                item.completed ? '已结束' : '进行中',
                                style: TextStyle(
                                  color: item.completed
                                      ? Colors.green
                                      : Colors.orange,
                                  fontSize: 11,
                                  fontWeight: FontWeight.w700,
                                ),
                              ),
                            ),
                            const SizedBox(width: 8),
                            Text(
                              '过程 ${item.processEntries.length}',
                              style: TextStyle(
                                color: palette.textSecondary,
                                fontSize: 12,
                              ),
                            ),
                          ],
                        ),
                        const SizedBox(height: 8),
                        Row(
                          mainAxisAlignment: MainAxisAlignment.end,
                          children: [
                            SizedBox(
                              height: 30,
                              child: IconButton(
                                onPressed: () => onToggleLock(item),
                                icon: Icon(
                                  item.locked
                                      ? Icons.lock_rounded
                                      : Icons.lock_open_rounded,
                                  size: 18,
                                ),
                                tooltip: item.locked ? '取消锁定' : '锁定',
                                padding: EdgeInsets.zero,
                                constraints: const BoxConstraints(
                                  minWidth: 30,
                                  minHeight: 30,
                                ),
                              ),
                            ),
                            const SizedBox(width: 4),
                            SizedBox(
                              height: 30,
                              child: IconButton(
                                onPressed: () => onReExecute(item),
                                icon: const Icon(
                                  Icons.play_arrow_rounded,
                                  size: 20,
                                ),
                                tooltip: '再次执行',
                                padding: EdgeInsets.zero,
                                constraints: const BoxConstraints(
                                  minWidth: 30,
                                  minHeight: 30,
                                ),
                              ),
                            ),
                          ],
                        ),
                      ],
                    ),
                  ),
                ),
              );
            },
          ),
        ],
      ),
    );
  }

  String? _resolveSelectedValue(List<String> items, String? selectedValue) {
    final selected = selectedValue?.trim() ?? '';
    if (selected.isEmpty) {
      return null;
    }
    return items.contains(selected) ? selected : null;
  }

  String _modeLabel(CodegenLaunchMode mode) {
    switch (mode) {
      case CodegenLaunchMode.code:
        return '编码';
      case CodegenLaunchMode.deploy:
        return '发布';
      case CodegenLaunchMode.backup:
        return '备份';
    }
  }

  Color _modeColor(CodegenLaunchMode mode) {
    switch (mode) {
      case CodegenLaunchMode.code:
        return Colors.blue;
      case CodegenLaunchMode.deploy:
        return Colors.green;
      case CodegenLaunchMode.backup:
        return Colors.deepPurple;
    }
  }

  String _formatTime(DateTime timestamp) {
    final hh = timestamp.hour.toString().padLeft(2, '0');
    final mm = timestamp.minute.toString().padLeft(2, '0');
    final ss = timestamp.second.toString().padLeft(2, '0');
    return '$hh:$mm:$ss';
  }
}
