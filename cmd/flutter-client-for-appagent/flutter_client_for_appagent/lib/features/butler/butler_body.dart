import 'package:flutter/material.dart';

/// 管家面板配色（与 CodegenBodyPalette 同模式，避免依赖主工程主题类型）。
class ButlerBodyPalette {
  const ButlerBodyPalette({
    required this.backgroundTop,
    required this.backgroundBottom,
    required this.surface,
    required this.surfaceSoft,
    required this.border,
    required this.accent,
    required this.textPrimary,
    required this.textSecondary,
    required this.textMuted,
    required this.success,
    required this.error,
  });

  final Color backgroundTop;
  final Color backgroundBottom;
  final Color surface;
  final Color surfaceSoft;
  final Color border;
  final Color accent;
  final Color textPrimary;
  final Color textSecondary;
  final Color textMuted;
  final Color success;
  final Color error;
}

/// 目标摘要条目（由 RawGetCurrentGoals 结果解析而来）。
class ButlerGoalSummary {
  const ButlerGoalSummary({
    required this.level,
    required this.period,
    required this.overview,
    required this.progress,
    required this.status,
    required this.taskCount,
  });

  final String level;
  final String period;
  final String overview;
  final int progress;
  final String status;
  final int taskCount;
}

/// 私人管家面板：目标概览 + 记忆审阅（可编辑）。
class ButlerBody extends StatelessWidget {
  const ButlerBody({
    super.key,
    required this.palette,
    required this.loading,
    required this.errorText,
    required this.affinityScore,
    required this.goals,
    required this.checkpoint,
    required this.memoryFiles,
    required this.selectedMemoryFile,
    required this.memoryEditorController,
    required this.memoryDirty,
    required this.onRefresh,
    required this.onOpenMemoryFile,
    required this.onSaveMemoryFile,
    required this.onMemoryChanged,
  });

  final ButlerBodyPalette palette;
  final bool loading;
  final String errorText;
  final int affinityScore;
  final List<ButlerGoalSummary> goals;
  final String checkpoint;
  final List<String> memoryFiles;
  final String selectedMemoryFile;
  final TextEditingController memoryEditorController;
  final bool memoryDirty;
  final VoidCallback onRefresh;
  final ValueChanged<String> onOpenMemoryFile;
  final VoidCallback onSaveMemoryFile;
  final VoidCallback onMemoryChanged;

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: BoxDecoration(
        gradient: LinearGradient(
          colors: <Color>[palette.backgroundTop, palette.backgroundBottom],
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
        ),
      ),
      child: SafeArea(
        child: RefreshIndicator(
          onRefresh: () async => onRefresh(),
          child: ListView(
            padding: const EdgeInsets.fromLTRB(14, 90, 14, 24),
            children: [
              _buildHeaderCard(),
              const SizedBox(height: 12),
              if (errorText.isNotEmpty) ...[
                Text(
                  errorText,
                  style: TextStyle(
                    color: palette.error,
                    fontWeight: FontWeight.w600,
                  ),
                ),
                const SizedBox(height: 12),
              ],
              _buildGoalsCard(),
              const SizedBox(height: 12),
              _buildCheckpointCard(),
              const SizedBox(height: 12),
              _buildMemoryCard(),
            ],
          ),
        ),
      ),
    );
  }

  Widget _card({required Widget child}) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: palette.surface.withValues(alpha: 0.92),
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: palette.border),
      ),
      child: child,
    );
  }

  Widget _sectionTitle(String title, {Widget? trailing}) {
    return Row(
      children: [
        Expanded(
          child: Text(
            title,
            style: TextStyle(
              color: palette.textPrimary,
              fontWeight: FontWeight.w700,
              fontSize: 15,
            ),
          ),
        ),
        ?trailing,
      ],
    );
  }

  Widget _buildHeaderCard() {
    return _card(
      child: Row(
        children: [
          Icon(Icons.workspace_premium_rounded, color: palette.accent, size: 34),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  '私人管家',
                  style: TextStyle(
                    color: palette.textPrimary,
                    fontWeight: FontWeight.w800,
                    fontSize: 17,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  '记忆可审阅 · 目标有判据 · 提醒不打扰',
                  style: TextStyle(color: palette.textMuted, fontSize: 12),
                ),
              ],
            ),
          ),
          Column(
            children: [
              Text(
                '$affinityScore',
                style: TextStyle(
                  color: palette.accent,
                  fontWeight: FontWeight.w800,
                  fontSize: 22,
                ),
              ),
              Text(
                '养成度',
                style: TextStyle(color: palette.textMuted, fontSize: 11),
              ),
            ],
          ),
          const SizedBox(width: 6),
          IconButton(
            tooltip: '刷新',
            onPressed: loading ? null : onRefresh,
            icon: loading
                ? const SizedBox(
                    width: 20,
                    height: 20,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  )
                : Icon(Icons.refresh_rounded, color: palette.textSecondary),
          ),
        ],
      ),
    );
  }

  static const Map<String, String> _levelLabels = {
    'daily': '今日',
    'weekly': '本周',
    'monthly': '本月',
    'yearly': '今年',
  };

  Widget _buildGoalsCard() {
    return _card(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _sectionTitle('目标概览'),
          const SizedBox(height: 10),
          if (goals.isEmpty)
            Text(
              '还没有目标。到聊天页对管家说出你的目标，它会帮你拆解并设定完成判据。',
              style: TextStyle(color: palette.textMuted, fontSize: 13),
            )
          else
            for (final goal in goals) ...[
              Row(
                children: [
                  Container(
                    padding: const EdgeInsets.symmetric(
                      horizontal: 8,
                      vertical: 2,
                    ),
                    decoration: BoxDecoration(
                      color: palette.surfaceSoft,
                      borderRadius: BorderRadius.circular(8),
                      border: Border.all(color: palette.border),
                    ),
                    child: Text(
                      _levelLabels[goal.level] ?? goal.level,
                      style: TextStyle(
                        color: palette.textSecondary,
                        fontSize: 11,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      goal.overview.isEmpty ? '(未填写目标描述)' : goal.overview,
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(
                        color: palette.textPrimary,
                        fontSize: 13,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                  ),
                  const SizedBox(width: 8),
                  Text(
                    '${goal.progress}%',
                    style: TextStyle(
                      color: goal.progress >= 100
                          ? palette.success
                          : palette.accent,
                      fontWeight: FontWeight.w800,
                      fontSize: 13,
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 6),
              ClipRRect(
                borderRadius: BorderRadius.circular(4),
                child: LinearProgressIndicator(
                  value: (goal.progress.clamp(0, 100)) / 100.0,
                  minHeight: 5,
                  backgroundColor: palette.surfaceSoft,
                ),
              ),
              const SizedBox(height: 12),
            ],
        ],
      ),
    );
  }

  Widget _buildCheckpointCard() {
    return _card(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _sectionTitle('当前关注点 (checkpoint)'),
          const SizedBox(height: 8),
          Text(
            checkpoint.trim().isEmpty ? '管家还没有写下关注点。' : checkpoint.trim(),
            style: TextStyle(
              color: palette.textSecondary,
              fontSize: 13,
              height: 1.5,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildMemoryCard() {
    return _card(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _sectionTitle(
            '记忆审阅',
            trailing: memoryDirty
                ? FilledButton.icon(
                    onPressed: onSaveMemoryFile,
                    icon: const Icon(Icons.save_outlined, size: 18),
                    label: const Text('保存'),
                  )
                : null,
          ),
          const SizedBox(height: 4),
          Text(
            '管家记住的一切都在这里，可直接修改或删除。',
            style: TextStyle(color: palette.textMuted, fontSize: 12),
          ),
          const SizedBox(height: 10),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              for (final file in memoryFiles)
                ChoiceChip(
                  label: Text(file, style: const TextStyle(fontSize: 12)),
                  selected: file == selectedMemoryFile,
                  onSelected: (_) => onOpenMemoryFile(file),
                ),
            ],
          ),
          if (selectedMemoryFile.isNotEmpty) ...[
            const SizedBox(height: 12),
            TextField(
              controller: memoryEditorController,
              minLines: 6,
              maxLines: 18,
              onChanged: (_) => onMemoryChanged(),
              style: TextStyle(
                color: palette.textPrimary,
                fontSize: 13,
                fontFamily: 'monospace',
                height: 1.5,
              ),
              decoration: InputDecoration(
                filled: true,
                fillColor: palette.surfaceSoft,
                border: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(12),
                  borderSide: BorderSide(color: palette.border),
                ),
                hintText: '（空文件）',
              ),
            ),
          ],
        ],
      ),
    );
  }
}
