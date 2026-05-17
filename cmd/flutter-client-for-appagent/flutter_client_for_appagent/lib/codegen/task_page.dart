import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';

import 'models.dart';

class CodegenTaskPage extends StatelessWidget {
  const CodegenTaskPage({
    super.key,
    required this.historyListenable,
    required this.itemId,
    required this.onReExecute,
    required this.onApply,
    required this.onToggleLock,
  });

  final ValueListenable<List<CodegenHistoryItem>> historyListenable;
  final String itemId;
  final ValueChanged<CodegenHistoryItem> onReExecute;
  final ValueChanged<CodegenHistoryItem> onApply;
  final ValueChanged<CodegenHistoryItem> onToggleLock;

  @override
  Widget build(BuildContext context) {
    return ValueListenableBuilder<List<CodegenHistoryItem>>(
      valueListenable: historyListenable,
      builder: (context, history, _) {
        final item = _findItem(history);
        if (item == null) {
          return Scaffold(
            appBar: AppBar(title: const Text('任务详情')),
            body: const Center(child: Text('任务记录不存在或已被清理。')),
          );
        }
        return _CodegenTaskDetails(
          item: item,
          onReExecute: onReExecute,
          onApply: onApply,
          onToggleLock: onToggleLock,
        );
      },
    );
  }

  CodegenHistoryItem? _findItem(List<CodegenHistoryItem> history) {
    for (final item in history) {
      if (item.id == itemId) {
        return item;
      }
    }
    return null;
  }
}

class _CodegenTaskDetails extends StatefulWidget {
  const _CodegenTaskDetails({
    required this.item,
    required this.onReExecute,
    required this.onApply,
    required this.onToggleLock,
  });

  final CodegenHistoryItem item;
  final ValueChanged<CodegenHistoryItem> onReExecute;
  final ValueChanged<CodegenHistoryItem> onApply;
  final ValueChanged<CodegenHistoryItem> onToggleLock;

  @override
  State<_CodegenTaskDetails> createState() => _CodegenTaskDetailsState();
}

class _CodegenTaskDetailsState extends State<_CodegenTaskDetails> {
  final ScrollController _scrollController = ScrollController();

  @override
  void dispose() {
    _scrollController.dispose();
    super.dispose();
  }

  void _scrollToBottom() {
    if (!_scrollController.hasClients) {
      return;
    }
    final target = _scrollController.position.maxScrollExtent;
    _scrollController.animateTo(
      target,
      duration: const Duration(milliseconds: 260),
      curve: Curves.easeOutCubic,
    );
  }

  @override
  Widget build(BuildContext context) {
    final item = widget.item;
    final details = CodegenHistoryCommandDetails.parse(item);
    final theme = Theme.of(context);
    final color = _modeColor(item.mode);
    return Scaffold(
      appBar: AppBar(
        title: Text('${_modeLabel(item.mode)}任务'),
        actions: [
          IconButton(
            onPressed: () => widget.onToggleLock(item),
            icon: Icon(
              item.locked ? Icons.lock_rounded : Icons.lock_open_rounded,
            ),
            tooltip: item.locked ? '取消锁定' : '锁定',
          ),
          IconButton(
            onPressed: () => widget.onReExecute(item),
            icon: const Icon(Icons.play_arrow_rounded),
            tooltip: '再次执行',
          ),
        ],
      ),
      body: SafeArea(
        child: Scrollbar(
          controller: _scrollController,
          thumbVisibility: true,
          interactive: true,
          child: ListView(
            controller: _scrollController,
            padding: const EdgeInsets.fromLTRB(16, 12, 24, 88),
            children: [
              Row(
                children: [
                  _StatusPill(
                    label: _modeLabel(item.mode),
                    color: color,
                    icon: item.mode == CodegenLaunchMode.deploy
                        ? Icons.rocket_launch_rounded
                        : Icons.terminal_rounded,
                  ),
                  const SizedBox(width: 8),
                  _StatusPill(
                    label: item.completed ? '已结束' : '进行中',
                    color: item.completed ? Colors.green : Colors.orange,
                    icon: item.completed
                        ? Icons.check_circle_rounded
                        : Icons.sync_rounded,
                  ),
                ],
              ),
              const SizedBox(height: 16),
              Text(
                _formatExactTime(item.timestamp),
                style: theme.textTheme.bodySmall?.copyWith(
                  color: theme.colorScheme.onSurfaceVariant,
                ),
              ),
              const SizedBox(height: 16),
              if (details.mode != CodegenLaunchMode.backup)
                _DetailBlock(label: '项目', value: details.projectQualifiedName),
              if (details.mode == CodegenLaunchMode.backup)
                _DetailBlock(
                  label: '备份类型',
                  value: details.requestText == 'full' ? '全量备份' : '增量备份',
                ),
              if (details.mode == CodegenLaunchMode.code)
                _DetailBlock(
                  label: '工具',
                  value: details.tool.isEmpty ? '默认' : details.tool,
                ),
              if (details.mode == CodegenLaunchMode.code &&
                  details.claudeSettings.isNotEmpty)
                _DetailBlock(label: 'Settings', value: details.claudeSettings),
              if (details.mode == CodegenLaunchMode.code)
                _DetailBlock(
                  label: '自动发布',
                  value: details.autoDeploy ? '是' : '否',
                ),
              if (details.mode == CodegenLaunchMode.code)
                _DetailBlock(
                  label: '继续上次会话',
                  value: details.resumeLastSession ? '是' : '否',
                ),
              if (details.mode == CodegenLaunchMode.code)
                _DetailBlock(label: '需求', value: details.requestText),
              if (details.mode == CodegenLaunchMode.deploy)
                _DetailBlock(
                  label: '部署目标',
                  value: details.target.isEmpty ? '未指定' : details.target,
                ),
              if (details.mode == CodegenLaunchMode.deploy)
                _DetailBlock(
                  label: '仅打包',
                  value: details.packOnly ? '是' : '否',
                ),
              if (details.mode == CodegenLaunchMode.deploy)
                _DetailBlock(
                  label: '附加参数',
                  value: details.extraArgs.isEmpty ? '无' : details.extraArgs,
                ),
              _DetailBlock(label: '完整命令', value: item.command, mono: true),
              _ProcessTimeline(entries: item.processEntries),
              const SizedBox(height: 12),
              FilledButton.icon(
                onPressed: () => widget.onReExecute(item),
                icon: const Icon(Icons.play_arrow_rounded),
                label: const Text('再次执行'),
              ),
              const SizedBox(height: 8),
              OutlinedButton.icon(
                onPressed: item.mode == CodegenLaunchMode.backup
                    ? null
                    : () => widget.onApply(item),
                icon: const Icon(Icons.edit_note_rounded),
                label: const Text('回填到表单'),
              ),
            ],
          ),
        ),
      ),
      floatingActionButton: FloatingActionButton.small(
        onPressed: _scrollToBottom,
        tooltip: '到底部',
        child: const Icon(Icons.vertical_align_bottom_rounded),
      ),
    );
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
}

class _StatusPill extends StatelessWidget {
  const _StatusPill({
    required this.label,
    required this.color,
    required this.icon,
  });

  final String label;
  final Color color;
  final IconData icon;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.14),
        borderRadius: BorderRadius.circular(6),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 16, color: color),
          const SizedBox(width: 6),
          Text(
            label,
            style: TextStyle(color: color, fontWeight: FontWeight.w700),
          ),
        ],
      ),
    );
  }
}

class _DetailBlock extends StatelessWidget {
  const _DetailBlock({
    required this.label,
    required this.value,
    this.mono = false,
  });

  final String label;
  final String value;
  final bool mono;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final displayValue = value.trim().isEmpty ? '未识别' : value.trim();
    return Padding(
      padding: const EdgeInsets.only(bottom: 14),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            label,
            style: theme.textTheme.labelMedium?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
              fontWeight: FontWeight.w700,
            ),
          ),
          const SizedBox(height: 6),
          Container(
            width: double.infinity,
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: theme.colorScheme.surfaceContainerHighest,
              borderRadius: BorderRadius.circular(8),
              border: Border.all(color: theme.colorScheme.outlineVariant),
            ),
            child: SelectableText(
              displayValue,
              style: theme.textTheme.bodyMedium?.copyWith(
                height: 1.45,
                fontFamily: mono ? 'monospace' : null,
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _ProcessTimeline extends StatelessWidget {
  const _ProcessTimeline({required this.entries});

  final List<CodegenProcessEntry> entries;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    if (entries.isEmpty) {
      return _DetailBlock(label: '执行过程', value: '暂无过程消息。');
    }
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          '执行过程',
          style: theme.textTheme.labelMedium?.copyWith(
            color: theme.colorScheme.onSurfaceVariant,
            fontWeight: FontWeight.w700,
          ),
        ),
        const SizedBox(height: 8),
        ...entries.map((entry) {
          return Container(
            width: double.infinity,
            margin: const EdgeInsets.only(bottom: 8),
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: theme.colorScheme.surface,
              borderRadius: BorderRadius.circular(8),
              border: Border.all(color: theme.colorScheme.outlineVariant),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  '${_formatClock(entry.timestamp)} · ${entry.origin}',
                  style: theme.textTheme.labelSmall?.copyWith(
                    color: theme.colorScheme.onSurfaceVariant,
                  ),
                ),
                const SizedBox(height: 6),
                SelectableText(
                  entry.content,
                  style: theme.textTheme.bodySmall?.copyWith(
                    height: 1.45,
                    fontFamily: 'monospace',
                  ),
                ),
              ],
            ),
          );
        }),
      ],
    );
  }
}

String _formatExactTime(DateTime timestamp) {
  return '${timestamp.year}-${timestamp.month.toString().padLeft(2, '0')}-'
      '${timestamp.day.toString().padLeft(2, '0')} '
      '${_formatClock(timestamp)}';
}

String _formatClock(DateTime timestamp) {
  return '${timestamp.hour.toString().padLeft(2, '0')}:'
      '${timestamp.minute.toString().padLeft(2, '0')}:'
      '${timestamp.second.toString().padLeft(2, '0')}';
}
