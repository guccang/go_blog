// ignore_for_file: invalid_use_of_protected_member
part of '../../main.dart';

extension _ChatPageStateUiSections on _ChatPageState {
  AppPalette get _palette => context.appPalette;

  Color get _connectionColor {
    final palette = _palette;
    if (_connected) {
      return palette.success;
    }
    if (_connecting || _loggingIn) {
      return palette.warning;
    }
    return palette.error;
  }

  String get _connectionLabel {
    if (_connected) {
      return 'Connected';
    }
    if (_connecting) {
      return 'Connecting';
    }
    if (_sessionToken.isEmpty && _refreshToken.isEmpty) {
      return 'Login required';
    }
    return 'Offline';
  }

  Widget _buildStatusChip({
    required IconData icon,
    required String label,
    required Color color,
  }) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(999),
        border: Border.all(color: color.withValues(alpha: 0.18)),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 16, color: color),
          const SizedBox(width: 6),
          Text(
            label,
            style: TextStyle(
              color: color,
              fontWeight: FontWeight.w700,
              fontSize: 12,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildCortanaChatSettings() {
    final palette = _palette;
    return Container(
      decoration: BoxDecoration(
        color: palette.surfaceMuted.withValues(alpha: 0.5),
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: palette.border.withValues(alpha: 0.55)),
      ),
      child: ExpansionTile(
        initiallyExpanded: _cortanaChatSettingsExpanded,
        onExpansionChanged: (v) {
          setState(() => _cortanaChatSettingsExpanded = v);
        },
        title: Text(
          'Cortana 设置',
          style: TextStyle(
            fontWeight: FontWeight.w600,
            color: palette.textPrimary,
          ),
        ),
        subtitle: Text(
          _cortanaChatSettingsExpanded ? '点击收起' : '默认折叠，点击配置',
          style: TextStyle(fontSize: 12, color: palette.textMuted),
        ),
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(12, 0, 12, 12),
            child: _buildCortanaChatSettingsContent(),
          ),
        ],
      ),
    );
  }

  Widget _buildCortanaChatSettingsContent() {
    final palette = _palette;
    final enabled = _cortanaEnabled;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SwitchListTile.adaptive(
          value: _cortanaEnabled,
          dense: true,
          contentPadding: EdgeInsets.zero,
          title: const Text('启用 Cortana'),
          subtitle: const Text('允许服务端为当前账号保持 Cortana 会话'),
          onChanged: (v) =>
              _applyCortanaSettings(_cortanaSettings.copyWith(enabled: v)),
        ),
        SwitchListTile.adaptive(
          value: _cortanaAllowFullAccess,
          dense: true,
          contentPadding: EdgeInsets.zero,
          title: const Text('允许全量感知'),
          subtitle: const Text('开放待办、锻炼、阅读、年度目标等数据'),
          onChanged: !enabled
              ? null
              : (v) => _applyCortanaSettings(
                  _cortanaSettings.copyWith(allowFullAccess: v),
                ),
        ),
        SwitchListTile.adaptive(
          value: _cortanaAutoPlay,
          dense: true,
          contentPadding: EdgeInsets.zero,
          title: const Text('主动播报自动播放'),
          subtitle: const Text('收到主动互动后直接语音播报'),
          onChanged: !enabled
              ? null
              : (v) => _applyCortanaSettings(
                  _cortanaSettings.copyWith(autoPlay: v),
                ),
        ),
        SwitchListTile.adaptive(
          value: _cortanaVoiceWakeEnabled,
          dense: true,
          contentPadding: EdgeInsets.zero,
          title: const Text('语音唤醒'),
          subtitle: Text(
            '前台持续监听"${_cortanaWakePhrase.trim().isEmpty ? '嗨 Cortana' : _cortanaWakePhrase.trim()}"后进入语音对话',
          ),
          onChanged: !enabled
              ? null
              : (v) => _applyCortanaSettings(
                  _cortanaSettings.copyWith(voiceWakeEnabled: v),
                ),
        ),
        Container(
          width: double.infinity,
          margin: const EdgeInsets.only(bottom: 8),
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
          decoration: BoxDecoration(
            color: palette.surfaceSoft,
            borderRadius: BorderRadius.circular(12),
            border: Border.all(color: palette.border),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                _cortanaWakeStatusLabel,
                style: TextStyle(
                  color: _cortanaWakeListening || _cortanaWakeAwaitingCommand
                      ? palette.accent
                      : palette.textPrimary,
                  fontWeight: FontWeight.w600,
                ),
              ),
              const SizedBox(height: 3),
              Text(
                _cortanaWakeStatusDetail,
                style: TextStyle(
                  color: palette.textMuted,
                  fontSize: 12,
                  height: 1.3,
                ),
              ),
            ],
          ),
        ),
        if (_cortanaVoiceWakeEnabled)
          TextField(
            controller: _cortanaWakePhraseCtrl,
            decoration: const InputDecoration(
              labelText: '唤醒词',
              hintText: '嗨 Cortana',
              isDense: true,
            ),
            onChanged: (v) => _applyCortanaSettings(
              _cortanaSettings.copyWith(
                wakePhrase: v.trim().isEmpty ? '嗨 Cortana' : v.trim(),
              ),
            ),
            onSubmitted: (v) => _applyCortanaSettings(
              _cortanaSettings.copyWith(
                wakePhrase: v.trim().isEmpty ? '嗨 Cortana' : v.trim(),
              ),
            ),
          ),
        const SizedBox(height: 6),
        DropdownButtonFormField<String>(
          initialValue: _cortanaProactiveMode,
          decoration: const InputDecoration(labelText: '主动模式', isDense: true),
          items: const [
            DropdownMenuItem(value: 'high', child: Text('High')),
            DropdownMenuItem(value: 'normal', child: Text('Normal')),
            DropdownMenuItem(value: 'low', child: Text('Low')),
          ],
          onChanged: !enabled
              ? null
              : (v) {
                  if (v == null || v.trim().isEmpty) return;
                  _applyCortanaSettings(
                    _cortanaSettings.copyWith(proactiveMode: v.trim()),
                  );
                },
        ),
        const SizedBox(height: 12),
        Text(
          '高频触发时间',
          style: TextStyle(
            color: palette.textPrimary,
            fontWeight: FontWeight.w600,
            fontSize: 14,
          ),
        ),
        const SizedBox(height: 4),
        Text(
          '在此时间段内 Cortana 主动播报频率更高',
          style: TextStyle(color: palette.textSecondary, fontSize: 12),
        ),
        const SizedBox(height: 8),
        Row(
          children: [
            Expanded(
              child: _buildCortanaTimePicker(
                label: '开始',
                hour: _cortanaHighFreqStartHour,
                minute: _cortanaHighFreqStartMinute,
                onChanged: (h, m) => _applyCortanaSettings(
                  _cortanaSettings.copyWith(
                    highFreqStartHour: h,
                    highFreqStartMinute: m,
                  ),
                ),
              ),
            ),
            const Padding(
              padding: EdgeInsets.symmetric(horizontal: 8),
              child: Text('—'),
            ),
            Expanded(
              child: _buildCortanaTimePicker(
                label: '结束',
                hour: _cortanaHighFreqEndHour,
                minute: _cortanaHighFreqEndMinute,
                onChanged: (h, m) => _applyCortanaSettings(
                  _cortanaSettings.copyWith(
                    highFreqEndHour: h,
                    highFreqEndMinute: m,
                  ),
                ),
              ),
            ),
          ],
        ),
        const SizedBox(height: 12),
        _buildCortanaLive2dSettings(),
        const SizedBox(height: 12),
        Text(
          '人设配置',
          style: TextStyle(
            color: palette.textPrimary,
            fontWeight: FontWeight.w600,
            fontSize: 14,
          ),
        ),
        const SizedBox(height: 8),
        TextField(
          controller: _cortanaPersonaNameCtrl,
          decoration: const InputDecoration(
            labelText: '名称',
            hintText: 'Cortana',
            isDense: true,
          ),
          onChanged: (v) => _applyCortanaSettings(
            _cortanaSettings.copyWith(personaName: v.trim()),
          ),
        ),
        const SizedBox(height: 8),
        TextField(
          controller: _cortanaOwnerTitleCtrl,
          decoration: const InputDecoration(
            labelText: '对用户称呼',
            hintText: '主人',
            isDense: true,
          ),
          onChanged: (v) => _applyCortanaSettings(
            _cortanaSettings.copyWith(ownerTitle: v.trim()),
          ),
        ),
        const SizedBox(height: 8),
        TextField(
          controller: _cortanaPersonaDescCtrl,
          maxLines: 3,
          decoration: const InputDecoration(
            labelText: '人设描述',
            hintText: '例如：你是一个友好、乐于助人的 AI 助手...',
            isDense: true,
            alignLabelWithHint: true,
          ),
          onChanged: (v) => _applyCortanaSettings(
            _cortanaSettings.copyWith(personaDescription: v.trim()),
          ),
        ),
      ],
    );
  }

  Widget _buildCortanaLive2dSettings() {
    final palette = _palette;
    final selectedModel = _findCortanaLive2dModel(
      _selectedCortanaLive2dModelId,
    );
    final selectedLabel = _selectedCortanaLive2dModelId.isEmpty
        ? '默认 Haru'
        : selectedModel?.name ?? '已安装形象';
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Live2D 形象',
          style: TextStyle(
            color: palette.textPrimary,
            fontWeight: FontWeight.w600,
            fontSize: 14,
          ),
        ),
        const SizedBox(height: 4),
        Text(
          '当前: $selectedLabel',
          style: TextStyle(color: palette.textSecondary, fontSize: 12),
        ),
        const SizedBox(height: 8),
        DropdownButtonFormField<String>(
          initialValue: _selectedCortanaLive2dModelId,
          decoration: const InputDecoration(labelText: '切换形象', isDense: true),
          items: [
            const DropdownMenuItem(value: '', child: Text('默认 Haru')),
            for (final model in _cortanaLive2dModels)
              DropdownMenuItem(value: model.id, child: Text(model.name)),
          ],
          onChanged: (value) {
            if (value == null) {
              return;
            }
            unawaited(_selectCortanaLive2dModel(value));
          },
        ),
        const SizedBox(height: 8),
        TextField(
          controller: _cortanaLive2dUrlCtrl,
          decoration: const InputDecoration(
            labelText: 'Live2D 资源 URL / 本机路径',
            hintText: 'file:///path/model.zip 或 https://.../model.zip',
            isDense: true,
          ),
          keyboardType: TextInputType.url,
        ),
        const SizedBox(height: 8),
        Row(
          children: [
            Expanded(
              child: FilledButton.icon(
                onPressed: _cortanaLive2dDownloading
                    ? null
                    : _downloadCortanaLive2dFromResourceUrl,
                icon: _cortanaLive2dDownloading
                    ? const SizedBox(
                        width: 16,
                        height: 16,
                        child: CircularProgressIndicator(strokeWidth: 2),
                      )
                    : const Icon(Icons.download_rounded),
                label: Text(_cortanaLive2dDownloading ? '下载中' : '下载并切换'),
              ),
            ),
          ],
        ),
        if (_cortanaLive2dDownloading ||
            (_cortanaLive2dDownloadProgress > 0 &&
                _cortanaLive2dDownloadProgress < 1))
          Padding(
            padding: const EdgeInsets.only(top: 8),
            child: LinearProgressIndicator(
              value: _cortanaLive2dDownloadProgress > 0
                  ? _cortanaLive2dDownloadProgress.clamp(0.0, 1.0)
                  : null,
            ),
          ),
        if (_cortanaLive2dDownloadError.isNotEmpty)
          Padding(
            padding: const EdgeInsets.only(top: 8),
            child: Text(
              _cortanaLive2dDownloadError,
              style: TextStyle(color: palette.error, fontSize: 12),
            ),
          ),
        if (_cortanaLive2dModels.isNotEmpty) ...[
          const SizedBox(height: 8),
          Container(
            decoration: BoxDecoration(
              color: palette.surfaceMuted.withValues(alpha: 0.42),
              borderRadius: BorderRadius.circular(8),
              border: Border.all(color: palette.border.withValues(alpha: 0.5)),
            ),
            child: ExpansionTile(
              key: const PageStorageKey<String>('cortana-live2d-model-list'),
              initiallyExpanded: false,
              tilePadding: const EdgeInsets.symmetric(horizontal: 8),
              childrenPadding: const EdgeInsets.fromLTRB(8, 0, 8, 8),
              title: Text('已下载形象 (${_cortanaLive2dModels.length})'),
              subtitle: Text(
                '默认折叠，点击展开管理',
                style: TextStyle(color: palette.textMuted, fontSize: 12),
              ),
              children: [
                for (final model in _cortanaLive2dModels)
                  ListTile(
                    dense: true,
                    contentPadding: EdgeInsets.zero,
                    leading: const Icon(Icons.face_retouching_natural),
                    title: Text(
                      model.name,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                    subtitle: Text(
                      model.sourceUrl,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                    trailing: IconButton(
                      tooltip: '删除',
                      onPressed: _cortanaLive2dDownloading
                          ? null
                          : () => unawaited(_deleteCortanaLive2dModel(model)),
                      icon: const Icon(Icons.delete_outline),
                    ),
                    onTap: () => unawaited(_selectCortanaLive2dModel(model.id)),
                  ),
              ],
            ),
          ),
        ],
      ],
    );
  }

  Widget _buildCortanaChatLogs() {
    final palette = _palette;
    final allLogEntries = List<FlutterClientLogEntry>.from(flutterClientLogs);
    final keyword = _cortanaLogKeywordCtrl.text.trim();
    final logEntries = _filterCortanaChatLogs(allLogEntries, keyword);
    return Container(
      decoration: BoxDecoration(
        color: palette.surfaceMuted.withValues(alpha: 0.5),
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: palette.border.withValues(alpha: 0.55)),
      ),
      child: ExpansionTile(
        initiallyExpanded: _cortanaChatLogsExpanded,
        onExpansionChanged: (v) {
          setState(() => _cortanaChatLogsExpanded = v);
        },
        title: Text(
          'Cortana 运行日志',
          style: TextStyle(
            fontWeight: FontWeight.w600,
            color: palette.textPrimary,
          ),
        ),
        subtitle: Text(
          allLogEntries.isEmpty
              ? '暂无 Flutter 客户端日志'
              : keyword.isEmpty
              ? '共 ${allLogEntries.length} 条 · 仅限客户端'
              : '显示 ${logEntries.length}/${allLogEntries.length} 条 · 关键字: $keyword',
          style: TextStyle(fontSize: 12, color: palette.textMuted),
        ),
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(12, 0, 12, 12),
            child: _buildCortanaChatLogsContent(
              entries: logEntries,
              totalCount: allLogEntries.length,
              keyword: keyword,
            ),
          ),
        ],
      ),
    );
  }

  List<FlutterClientLogEntry> _filterCortanaChatLogs(
    List<FlutterClientLogEntry> entries,
    String keyword,
  ) {
    final query = keyword.trim().toLowerCase();
    if (query.isEmpty) {
      return entries;
    }
    final terms = query
        .split(RegExp(r'\s+'))
        .where((term) => term.trim().isNotEmpty)
        .toList(growable: false);
    if (terms.isEmpty) {
      return entries;
    }
    return entries
        .where((entry) {
          final haystack = '${entry.timeLabel} ${entry.message}'.toLowerCase();
          return terms.every(haystack.contains);
        })
        .toList(growable: false);
  }

  void _setCortanaLogKeyword(String keyword) {
    _cortanaLogKeywordCtrl.text = keyword;
    _cortanaLogKeywordCtrl.selection = TextSelection.collapsed(
      offset: _cortanaLogKeywordCtrl.text.length,
    );
    if (mounted) {
      setState(() {});
    }
  }

  Widget _buildCortanaChatLogsContent({
    required List<FlutterClientLogEntry> entries,
    required int totalCount,
    required String keyword,
  }) {
    final palette = _palette;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Text(
              '客户端日志',
              style: TextStyle(
                fontWeight: FontWeight.w700,
                color: palette.textPrimary,
                fontSize: 13,
              ),
            ),
            const Spacer(),
            if (keyword.isNotEmpty)
              TextButton.icon(
                onPressed: () => _setCortanaLogKeyword(''),
                icon: const Icon(Icons.filter_alt_off_outlined, size: 16),
                label: const Text('清除筛选'),
                style: TextButton.styleFrom(foregroundColor: palette.textMuted),
              ),
            if (totalCount > 0)
              TextButton.icon(
                onPressed: () {
                  flutterClientLogs.clear();
                  if (mounted) setState(() {});
                },
                icon: const Icon(Icons.delete_outline, size: 16),
                label: const Text('清空'),
                style: TextButton.styleFrom(foregroundColor: palette.textMuted),
              ),
          ],
        ),
        const SizedBox(height: 8),
        TextField(
          controller: _cortanaLogKeywordCtrl,
          onChanged: (_) => setState(() {}),
          textInputAction: TextInputAction.search,
          decoration: InputDecoration(
            isDense: true,
            labelText: '日志关键字',
            hintText: '输入 语音唤醒 / Speech recognition / error',
            prefixIcon: const Icon(Icons.search_rounded, size: 18),
            suffixIcon: keyword.isEmpty
                ? null
                : IconButton(
                    tooltip: '清除筛选',
                    icon: const Icon(Icons.close_rounded, size: 18),
                    onPressed: () => _setCortanaLogKeyword(''),
                  ),
          ),
        ),
        const SizedBox(height: 8),
        Wrap(
          spacing: 8,
          runSpacing: 6,
          children: [
            ActionChip(
              avatar: const Icon(Icons.graphic_eq_rounded, size: 16),
              label: const Text('只看语音唤醒'),
              onPressed: () => _setCortanaLogKeyword('语音唤醒'),
            ),
            ActionChip(
              avatar: const Icon(Icons.mic_rounded, size: 16),
              label: const Text('Speech recognition'),
              onPressed: () => _setCortanaLogKeyword('Speech recognition'),
            ),
            ActionChip(
              avatar: const Icon(Icons.error_outline_rounded, size: 16),
              label: const Text('失败'),
              onPressed: () => _setCortanaLogKeyword('失败'),
            ),
          ],
        ),
        const SizedBox(height: 8),
        Container(
          width: double.infinity,
          constraints: const BoxConstraints(maxHeight: 300, minHeight: 120),
          decoration: BoxDecoration(
            color: palette.surfaceMuted.withValues(alpha: 0.92),
            borderRadius: BorderRadius.circular(12),
            border: Border.all(color: palette.border.withValues(alpha: 0.55)),
          ),
          child: ClipRRect(
            borderRadius: BorderRadius.circular(12),
            child: entries.isEmpty
                ? Center(
                    child: Text(
                      totalCount == 0
                          ? '暂无日志 · 操作 Cortana 对话时将自动采集'
                          : '没有匹配关键字的日志',
                      style: TextStyle(color: palette.textMuted, fontSize: 13),
                    ),
                  )
                : Scrollbar(
                    thumbVisibility: true,
                    child: ListView.builder(
                      padding: const EdgeInsets.all(10),
                      itemCount: entries.length,
                      itemBuilder: (context, index) {
                        final entry = entries[index];
                        return Padding(
                          padding: const EdgeInsets.only(bottom: 4),
                          child: Row(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(
                                entry.timeLabel,
                                style: TextStyle(
                                  fontFamily: 'monospace',
                                  fontSize: 11,
                                  color: palette.accent,
                                  fontWeight: FontWeight.w600,
                                ),
                              ),
                              const SizedBox(width: 8),
                              Expanded(
                                child: SelectableText(
                                  entry.message,
                                  style: TextStyle(
                                    fontFamily: 'monospace',
                                    fontSize: 12,
                                    height: 1.4,
                                    color: palette.textPrimary,
                                  ),
                                ),
                              ),
                            ],
                          ),
                        );
                      },
                    ),
                  ),
          ),
        ),
      ],
    );
  }

  Widget _buildCortanaTimePicker({
    required String label,
    required int hour,
    required int minute,
    required void Function(int hour, int minute)? onChanged,
  }) {
    final hh = hour.toString().padLeft(2, '0');
    final mm = minute.toString().padLeft(2, '0');
    return InkWell(
      onTap: onChanged == null
          ? null
          : () async {
              final time = await showTimePicker(
                context: context,
                initialTime: TimeOfDay(hour: hour, minute: minute),
              );
              if (time != null) {
                onChanged(time.hour, time.minute);
              }
            },
      borderRadius: BorderRadius.circular(8),
      child: InputDecorator(
        decoration: InputDecoration(labelText: label, isDense: true),
        child: Text('$hh:$mm', style: Theme.of(context).textTheme.bodyLarge),
      ),
    );
  }

  Widget _buildConfigItem({
    required IconData icon,
    required String label,
    required String value,
    required VoidCallback? onCopy,
  }) {
    final palette = _palette;
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
      decoration: BoxDecoration(
        color: palette.surfaceRaised,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: palette.border),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(icon, size: 18, color: palette.textSecondary),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  label,
                  style: TextStyle(
                    fontSize: 11,
                    fontWeight: FontWeight.w700,
                    color: palette.textMuted,
                  ),
                ),
                const SizedBox(height: 4),
                SelectionArea(
                  child: Text(
                    value.isEmpty ? '-' : value,
                    style: TextStyle(
                      fontSize: 13,
                      height: 1.35,
                      color: palette.textPrimary,
                    ),
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(width: 8),
          IconButton(
            onPressed: onCopy,
            tooltip: 'Copy $label',
            visualDensity: VisualDensity.compact,
            icon: const Icon(Icons.copy_rounded, size: 18),
          ),
        ],
      ),
    );
  }

  Widget _buildSherpaModelCard() {
    final palette = _palette;
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
      decoration: BoxDecoration(
        color: palette.surfaceRaised,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: palette.border),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(
                Icons.model_training_rounded,
                size: 18,
                color: palette.accent,
              ),
              const SizedBox(width: 10),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      'Sherpa-onnx 语音模型',
                      style: TextStyle(
                        fontSize: 11,
                        fontWeight: FontWeight.w700,
                        color: palette.textMuted,
                      ),
                    ),
                    const SizedBox(height: 4),
                    Text(
                      'SenseVoice 转写 + Zipformer 中文唤醒模型',
                      style: TextStyle(
                        fontSize: 12,
                        color: palette.textSecondary,
                      ),
                    ),
                  ],
                ),
              ),
              FutureBuilder<bool>(
                key: const ValueKey('sherpa_model_check'),
                future: _isSherpaModelDownloaded(),
                builder: (context, snapshot) {
                  final isDownloaded = snapshot.data ?? false;
                  if (_sherpaModelDownloading) {
                    return Container(
                      padding: const EdgeInsets.symmetric(
                        horizontal: 12,
                        vertical: 6,
                      ),
                      decoration: BoxDecoration(
                        color: palette.surfaceSoft,
                        borderRadius: BorderRadius.circular(12),
                        border: Border.all(color: palette.borderStrong),
                      ),
                      child: Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          SizedBox(
                            width: 14,
                            height: 14,
                            child: CircularProgressIndicator(
                              strokeWidth: 2,
                              value: _sherpaModelDownloadProgress,
                              backgroundColor: palette.surfaceMuted,
                            ),
                          ),
                          const SizedBox(width: 8),
                          Text(
                            '${(_sherpaModelDownloadProgress * 100).toStringAsFixed(0)}%',
                            style: TextStyle(
                              fontSize: 12,
                              fontWeight: FontWeight.w700,
                              color: palette.textPrimary,
                            ),
                          ),
                        ],
                      ),
                    );
                  }
                  if (isDownloaded) {
                    return Container(
                      padding: const EdgeInsets.symmetric(
                        horizontal: 12,
                        vertical: 6,
                      ),
                      decoration: BoxDecoration(
                        color: palette.success.withValues(alpha: 0.12),
                        borderRadius: BorderRadius.circular(12),
                        border: Border.all(
                          color: palette.success.withValues(alpha: 0.3),
                        ),
                      ),
                      child: Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Icon(
                            Icons.check_circle_rounded,
                            size: 14,
                            color: palette.success,
                          ),
                          const SizedBox(width: 6),
                          Text(
                            '已安装',
                            style: TextStyle(
                              fontSize: 12,
                              fontWeight: FontWeight.w700,
                              color: palette.success,
                            ),
                          ),
                        ],
                      ),
                    );
                  }
                  return FutureBuilder<bool>(
                    key: const ValueKey('sherpa_partial_check'),
                    future: _hasPartialSherpaDownload(),
                    builder: (context, partialSnapshot) {
                      final hasPartial = partialSnapshot.data ?? false;
                      return FilledButton.tonal(
                        onPressed: _sherpaModelDownloading
                            ? null
                            : _downloadAndExtractSherpaModels,
                        style: FilledButton.styleFrom(
                          backgroundColor: hasPartial
                              ? palette.warning
                              : palette.accent,
                          foregroundColor: _foregroundForColor(
                            hasPartial ? palette.warning : palette.accent,
                          ),
                          minimumSize: const Size(0, 32),
                          padding: const EdgeInsets.symmetric(horizontal: 12),
                          shape: RoundedRectangleBorder(
                            borderRadius: BorderRadius.circular(12),
                          ),
                        ),
                        child: Row(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            if (hasPartial) ...[
                              const Icon(Icons.play_arrow_rounded, size: 14),
                              const SizedBox(width: 4),
                            ],
                            Text(
                              hasPartial ? '继续下载' : '下载',
                              style: const TextStyle(fontSize: 12),
                            ),
                          ],
                        ),
                      );
                    },
                  );
                },
              ),
            ],
          ),
          const SizedBox(height: 8),
          TextField(
            controller: _sherpaDownloadUrlController,
            enabled: !_sherpaModelDownloading,
            decoration: const InputDecoration(
              labelText: 'ASR 手动下载地址（可选）',
              hintText: 'https://.../sherpa-onnx-sense-voice.tar.bz2',
              isDense: true,
            ),
            keyboardType: TextInputType.url,
            onChanged: (value) {
              unawaited(
                SharedPreferences.getInstance().then((prefs) async {
                  final url = value.trim();
                  if (url.isEmpty) {
                    await prefs.remove(_sherpaManualDownloadUrlKey);
                  } else {
                    await prefs.setString(_sherpaManualDownloadUrlKey, url);
                  }
                }),
              );
            },
          ),
          const SizedBox(height: 4),
          Text(
            '留空时使用内置下载源；填写后优先使用该地址。',
            style: TextStyle(fontSize: 11, color: palette.textSecondary),
          ),
          if (_sherpaModelDownloadError != null) ...[
            const SizedBox(height: 8),
            Container(
              padding: const EdgeInsets.all(8),
              decoration: BoxDecoration(
                color: palette.error.withValues(alpha: 0.1),
                borderRadius: BorderRadius.circular(8),
                border: Border.all(color: palette.error.withValues(alpha: 0.3)),
              ),
              child: Row(
                children: [
                  Icon(
                    Icons.error_outline_rounded,
                    size: 14,
                    color: palette.error,
                  ),
                  const SizedBox(width: 6),
                  Expanded(
                    child: Text(
                      _sherpaModelDownloadError!,
                      style: TextStyle(fontSize: 11, color: palette.error),
                    ),
                  ),
                  IconButton(
                    onPressed: () {
                      setState(() {
                        _sherpaModelDownloadError = null;
                      });
                    },
                    tooltip: '关闭',
                    visualDensity: VisualDensity.compact,
                    iconSize: 16,
                    icon: Icon(
                      Icons.close_rounded,
                      size: 14,
                      color: palette.error,
                    ),
                  ),
                ],
              ),
            ),
          ],
          if (_sherpaModelDownloadError != null ||
              _hasPartialSherpaDownloadSync()) ...[
            const SizedBox(height: 4),
            FutureBuilder<bool>(
              future: _hasPartialSherpaDownload(),
              builder: (context, snapshot) {
                final hasPartial = snapshot.data ?? false;
                if (!hasPartial) return const SizedBox.shrink();
                return TextButton.icon(
                  onPressed: _clearSherpaDownloadCache,
                  style: TextButton.styleFrom(
                    foregroundColor: palette.textSecondary,
                    padding: const EdgeInsets.symmetric(
                      horizontal: 8,
                      vertical: 4,
                    ),
                    tapTargetSize: MaterialTapTargetSize.shrinkWrap,
                  ),
                  icon: const Icon(Icons.delete_outline_rounded, size: 12),
                  label: const Text('清除缓存', style: TextStyle(fontSize: 11)),
                );
              },
            ),
          ],
        ],
      ),
    );
  }

  Widget _buildThemePresetCard() {
    final palette = _palette;
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 12),
      decoration: BoxDecoration(
        color: palette.surfaceRaised,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: palette.border),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(
                Icons.palette_outlined,
                size: 18,
                color: palette.textSecondary,
              ),
              const SizedBox(width: 10),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      'UI 主题色',
                      style: TextStyle(
                        fontSize: 11,
                        fontWeight: FontWeight.w700,
                        color: palette.textMuted,
                      ),
                    ),
                    const SizedBox(height: 4),
                    Text(
                      '切换整套 UI 的主色、背景和控件氛围',
                      style: TextStyle(
                        fontSize: 12,
                        color: palette.textSecondary,
                      ),
                    ),
                  ],
                ),
              ),
            ],
          ),
          const SizedBox(height: 12),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              for (final preset in kUiThemePresets)
                ChoiceChip(
                  selected: widget.themePreset.id == preset.id,
                  label: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Container(
                        width: 10,
                        height: 10,
                        decoration: BoxDecoration(
                          color: preset.accent,
                          borderRadius: BorderRadius.circular(999),
                        ),
                      ),
                      const SizedBox(width: 6),
                      Text(preset.label),
                    ],
                  ),
                  onSelected: (_) => widget.onThemePresetChanged(preset),
                ),
            ],
          ),
        ],
      ),
    );
  }

  bool _hasPartialSherpaDownloadSync() {
    // Quick sync check for UI rendering
    return _sherpaModelDownloadProgress > 0 &&
        _sherpaModelDownloadProgress < 1.0;
  }

  Future<void> _clearSherpaDownloadCache() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      await prefs.remove(_sherpaDownloadProgressKey);
      await prefs.remove(_sherpaDownloadBytesKey);
      await prefs.remove(_sherpaActiveDownloadUrlKey);

      for (final id in const <String>['asr', 'kws']) {
        final archiveFile = await _getSherpaArchiveFile(id);
        final partFile = await _getSherpaArchivePartFile(id);
        final tempModelDir = await _getSherpaExtractionTempDir(id);
        await _deleteFileIfExists(archiveFile);
        await _deleteFileIfExists(partFile);
        await _deleteDirectoryIfExists(tempModelDir);
        await prefs.remove('$_sherpaActiveDownloadUrlKey::$id');
        await prefs.remove('$_sherpaDownloadBytesKey::$id');
      }

      if (!mounted) {
        return;
      }
      setState(() {
        _sherpaModelDownloadProgress = 0.0;
        _sherpaModelDownloadError = null;
      });
      _appendSystem('已清除 Sherpa 模型下载缓存');
    } catch (err) {
      _appendSystem('清除缓存失败: $err');
    }
  }

  Widget _buildAttachmentMenuButton({required bool enabled}) {
    final palette = _palette;
    return PopupMenuButton<_AttachmentMenuAction>(
      enabled: enabled,
      tooltip: 'Attachments',
      onSelected: (action) => unawaited(_handleAttachmentMenuAction(action)),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(18)),
      color: palette.surfaceMuted,
      itemBuilder: (context) => [
        PopupMenuItem<_AttachmentMenuAction>(
          value: _AttachmentMenuAction.galleryImage,
          child: ListTile(
            contentPadding: EdgeInsets.zero,
            leading: Icon(
              Icons.photo_library_rounded,
              color: palette.textPrimary,
            ),
            title: Text(
              'Send Image',
              style: TextStyle(color: palette.textPrimary),
            ),
            subtitle: Text(
              'Choose from gallery',
              style: TextStyle(color: palette.textSecondary),
            ),
          ),
        ),
        PopupMenuItem<_AttachmentMenuAction>(
          value: _AttachmentMenuAction.cameraImage,
          child: ListTile(
            contentPadding: EdgeInsets.zero,
            leading: Icon(
              Icons.photo_camera_back_rounded,
              color: palette.textPrimary,
            ),
            title: Text(
              'Take Photo',
              style: TextStyle(color: palette.textPrimary),
            ),
            subtitle: Text(
              'Capture and send',
              style: TextStyle(color: palette.textSecondary),
            ),
          ),
        ),
        PopupMenuItem<_AttachmentMenuAction>(
          value: _AttachmentMenuAction.imageResource,
          child: ListTile(
            contentPadding: EdgeInsets.zero,
            leading: Icon(Icons.image_outlined, color: palette.textPrimary),
            title: Text(
              'Upload Image Asset',
              style: TextStyle(color: palette.textPrimary),
            ),
            subtitle: Text(
              'Store under image',
              style: TextStyle(color: palette.textSecondary),
            ),
          ),
        ),
        PopupMenuItem<_AttachmentMenuAction>(
          value: _AttachmentMenuAction.live2dResource,
          child: ListTile(
            contentPadding: EdgeInsets.zero,
            leading: Icon(
              Icons.face_retouching_natural_outlined,
              color: palette.textPrimary,
            ),
            title: Text(
              'Upload Live2D Asset',
              style: TextStyle(color: palette.textPrimary),
            ),
            subtitle: Text(
              'Zip or model file path',
              style: TextStyle(color: palette.textSecondary),
            ),
          ),
        ),
        PopupMenuItem<_AttachmentMenuAction>(
          value: _AttachmentMenuAction.fileResource,
          child: ListTile(
            contentPadding: EdgeInsets.zero,
            leading: Icon(
              Icons.upload_file_outlined,
              color: palette.textPrimary,
            ),
            title: Text(
              'Upload File Asset',
              style: TextStyle(color: palette.textPrimary),
            ),
            subtitle: Text(
              'Store under file',
              style: TextStyle(color: palette.textSecondary),
            ),
          ),
        ),
        PopupMenuItem<_AttachmentMenuAction>(
          value: _AttachmentMenuAction.browseResources,
          child: ListTile(
            contentPadding: EdgeInsets.zero,
            leading: Icon(
              Icons.inventory_2_outlined,
              color: palette.textPrimary,
            ),
            title: Text(
              'Browse Assets',
              style: TextStyle(color: palette.textPrimary),
            ),
            subtitle: Text(
              'Live2D, image, file',
              style: TextStyle(color: palette.textSecondary),
            ),
          ),
        ),
      ],
      child: AnimatedContainer(
        duration: const Duration(milliseconds: 160),
        height: 42,
        width: 42,
        decoration: BoxDecoration(
          color: enabled ? palette.surfaceSoft : palette.surfaceMuted,
          borderRadius: BorderRadius.circular(14),
          border: Border.all(
            color: enabled ? palette.borderStrong : palette.border,
          ),
        ),
        child: Icon(
          Icons.add_rounded,
          color: enabled ? palette.textPrimary : palette.textMuted,
        ),
      ),
    );
  }

  Widget _buildComposerIconButton({
    required IconData icon,
    required VoidCallback? onPressed,
    bool active = false,
    Color? activeColor,
  }) {
    final palette = _palette;
    final enabled = onPressed != null;
    final effectiveActiveColor = activeColor ?? palette.accent;
    return Material(
      color: Colors.transparent,
      child: InkWell(
        onTap: onPressed,
        borderRadius: BorderRadius.circular(14),
        child: AnimatedContainer(
          duration: const Duration(milliseconds: 160),
          height: 42,
          width: 42,
          decoration: BoxDecoration(
            color: active
                ? effectiveActiveColor
                : enabled
                ? palette.surfaceSoft
                : palette.surfaceMuted,
            borderRadius: BorderRadius.circular(14),
            border: Border.all(
              color: active
                  ? effectiveActiveColor.withValues(alpha: 0.9)
                  : enabled
                  ? palette.borderStrong
                  : palette.border,
            ),
          ),
          child: Icon(
            icon,
            color: active
                ? _foregroundForColor(effectiveActiveColor)
                : enabled
                ? palette.textPrimary
                : palette.textMuted,
            size: 21,
          ),
        ),
      ),
    );
  }

  Widget _buildChatControlsPanel() {
    final palette = _palette;
    final canLogin = !_loggingIn && !_configLoading && _clientConfig != null;
    final hasGroups = _groups.isNotEmpty;
    final groupsTabSelected = _currentGroupId.isNotEmpty;
    final showGroupTabs = hasGroups && _groups.length > 1 && _groupTabsExpanded;
    final controlsMaxHeight = math.min(
      MediaQuery.sizeOf(context).height * 0.52,
      440.0,
    );
    final compactButtonLabel = _controlsExpanded
        ? 'Hide'
        : (_sessionToken.isEmpty ? 'Login' : 'Controls');

    return Container(
      width: double.infinity,
      padding: const EdgeInsets.fromLTRB(12, 10, 12, 10),
      decoration: BoxDecoration(
        gradient: LinearGradient(
          colors: <Color>[palette.surface, palette.surfaceRaised],
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
        ),
        borderRadius: BorderRadius.circular(24),
        border: Border.all(color: palette.border),
        boxShadow: [
          BoxShadow(
            blurRadius: 18,
            color: Colors.black.withValues(alpha: 0.22),
            offset: const Offset(0, 10),
          ),
        ],
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.center,
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    SizedBox(
                      height: 44,
                      child: ListView(
                        scrollDirection: Axis.horizontal,
                        padding: const EdgeInsets.symmetric(vertical: 2),
                        children: [
                          ChoiceChip(
                            selected: !groupsTabSelected,
                            label: const Text('Direct'),
                            onSelected: (_) =>
                                unawaited(_switchToDirectScope()),
                          ),
                          if (hasGroups) ...[
                            const SizedBox(width: 8),
                            ChoiceChip(
                              selected: groupsTabSelected,
                              label: Row(
                                mainAxisSize: MainAxisSize.min,
                                children: [
                                  Text(
                                    _groups.length == 1
                                        ? 'Group'
                                        : 'Groups (${_groups.length})',
                                  ),
                                  if (_groups.length > 1) ...[
                                    const SizedBox(width: 4),
                                    Icon(
                                      showGroupTabs
                                          ? Icons.expand_less_rounded
                                          : Icons.expand_more_rounded,
                                      size: 16,
                                    ),
                                  ],
                                ],
                              ),
                              onSelected: (_) =>
                                  unawaited(_switchToGroupsTab()),
                            ),
                          ],
                        ],
                      ),
                    ),
                    if (showGroupTabs) ...[
                      const SizedBox(height: 8),
                      SizedBox(
                        height: 46,
                        child: ListView.separated(
                          scrollDirection: Axis.horizontal,
                          padding: const EdgeInsets.symmetric(vertical: 3),
                          itemCount: _groups.length,
                          separatorBuilder: (_, _) => const SizedBox(width: 8),
                          itemBuilder: (context, index) {
                            final group = _groups[index];
                            return ChoiceChip(
                              selected: _currentGroupId == group.id,
                              label: Text(
                                '${group.id} (${group.members.length})',
                              ),
                              onSelected: (_) =>
                                  unawaited(_switchToGroupScope(group.id)),
                            );
                          },
                        ),
                      ),
                    ],
                  ],
                ),
              ),
              const SizedBox(width: 8),
              OutlinedButton.icon(
                onPressed: () {
                  setState(() {
                    _controlsExpanded = !_controlsExpanded;
                  });
                },
                style: OutlinedButton.styleFrom(
                  foregroundColor: palette.textPrimary,
                  side: BorderSide(color: palette.borderStrong),
                  minimumSize: const Size(0, 40),
                  padding: const EdgeInsets.symmetric(horizontal: 12),
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(16),
                  ),
                ),
                icon: Icon(
                  _controlsExpanded
                      ? Icons.expand_less_rounded
                      : Icons.tune_rounded,
                ),
                label: Text(compactButtonLabel),
              ),
            ],
          ),
          AnimatedCrossFade(
            firstChild: const SizedBox.shrink(),
            secondChild: Padding(
              padding: const EdgeInsets.only(top: 10),
              child: ConstrainedBox(
                constraints: BoxConstraints(maxHeight: controlsMaxHeight),
                child: Scrollbar(
                  controller: _controlsScrollController,
                  thumbVisibility: true,
                  radius: const Radius.circular(999),
                  child: SingleChildScrollView(
                    controller: _controlsScrollController,
                    padding: const EdgeInsets.only(right: 6, bottom: 4),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        TextField(
                          controller: _userIdController,
                          decoration: const InputDecoration(
                            labelText: 'User ID',
                            hintText: 'demo-user',
                            prefixIcon: Icon(Icons.badge_outlined),
                            isDense: true,
                          ),
                        ),
                        const SizedBox(height: 10),
                        TextField(
                          controller: _passwordController,
                          obscureText: !_passwordVisible,
                          decoration: InputDecoration(
                            labelText: 'Password',
                            hintText: 'blog-agent password',
                            prefixIcon: const Icon(Icons.lock_outline_rounded),
                            isDense: true,
                            suffixIcon: IconButton(
                              onPressed: () {
                                setState(() {
                                  _passwordVisible = !_passwordVisible;
                                });
                              },
                              icon: Icon(
                                _passwordVisible
                                    ? Icons.visibility_off_outlined
                                    : Icons.visibility_outlined,
                              ),
                            ),
                          ),
                        ),
                        const SizedBox(height: 10),
                        TextField(
                          controller: _groupIdController,
                          decoration: const InputDecoration(
                            labelText: 'Group ID',
                            hintText: 'party-01',
                            prefixIcon: Icon(Icons.groups_2_outlined),
                            isDense: true,
                          ),
                        ),
                        const SizedBox(height: 12),
                        Wrap(
                          spacing: 10,
                          runSpacing: 10,
                          children: [
                            FilledButton.icon(
                              onPressed: canLogin ? _login : null,
                              style: FilledButton.styleFrom(
                                backgroundColor: palette.accent,
                                foregroundColor: _foregroundForColor(
                                  palette.accent,
                                ),
                                minimumSize: const Size(120, 48),
                                padding: const EdgeInsets.symmetric(
                                  horizontal: 14,
                                ),
                                shape: RoundedRectangleBorder(
                                  borderRadius: BorderRadius.circular(18),
                                ),
                              ),
                              icon: Icon(
                                _sessionToken.isEmpty
                                    ? Icons.login
                                    : Icons.refresh_rounded,
                              ),
                              label: Text(
                                _sessionToken.isEmpty ? 'Login' : 'Re-login',
                              ),
                            ),
                            OutlinedButton.icon(
                              onPressed: _connected || _connecting
                                  ? _disconnectWs
                                  : null,
                              style: OutlinedButton.styleFrom(
                                foregroundColor: palette.textPrimary,
                                minimumSize: const Size(120, 48),
                                side: BorderSide(color: palette.borderStrong),
                                shape: RoundedRectangleBorder(
                                  borderRadius: BorderRadius.circular(18),
                                ),
                              ),
                              icon: const Icon(Icons.link_off_rounded),
                              label: const Text('Disconnect'),
                            ),
                            OutlinedButton.icon(
                              onPressed:
                                  _sessionToken.isEmpty && _refreshToken.isEmpty
                                  ? null
                                  : _logout,
                              style: OutlinedButton.styleFrom(
                                foregroundColor: palette.textPrimary,
                                minimumSize: const Size(120, 48),
                                side: BorderSide(color: palette.borderStrong),
                                shape: RoundedRectangleBorder(
                                  borderRadius: BorderRadius.circular(18),
                                ),
                              ),
                              icon: const Icon(Icons.logout_rounded),
                              label: const Text('Logout'),
                            ),
                          ],
                        ),
                        const SizedBox(height: 10),
                        Wrap(
                          spacing: 8,
                          runSpacing: 8,
                          children: [
                            FilledButton.tonal(
                              onPressed: _sessionToken.isEmpty
                                  ? null
                                  : () => _mutateGroup('create'),
                              child: const Text('Create'),
                            ),
                            FilledButton.tonal(
                              onPressed: _sessionToken.isEmpty
                                  ? null
                                  : () => _mutateGroup('join'),
                              child: const Text('Join'),
                            ),
                            OutlinedButton(
                              onPressed: _sessionToken.isEmpty
                                  ? null
                                  : () => _mutateGroup('leave'),
                              child: const Text('Leave'),
                            ),
                          ],
                        ),
                      ],
                    ),
                  ),
                ),
              ),
            ),
            crossFadeState: _controlsExpanded
                ? CrossFadeState.showSecond
                : CrossFadeState.showFirst,
            duration: const Duration(milliseconds: 180),
          ),
        ],
      ),
    );
  }

  Widget _buildComposer() {
    final palette = _palette;
    final canInteract = !(_sending || _recording || _transcribingVoice);
    final showSendButton = !_voiceInputMode && _composerHasText;    return Container(
      margin: const EdgeInsets.fromLTRB(10, 0, 10, 10),
      padding: const EdgeInsets.fromLTRB(10, 10, 10, 10),
      decoration: BoxDecoration(
        gradient: LinearGradient(
          colors: <Color>[palette.surface, palette.surfaceMuted],
          begin: Alignment.topCenter,
          end: Alignment.bottomCenter,
        ),
        borderRadius: BorderRadius.circular(24),
        border: Border.all(color: palette.border),
        boxShadow: [
          BoxShadow(
            blurRadius: 24,
            color: Colors.black.withValues(alpha: 0.18),
            offset: const Offset(0, 12),
          ),
        ],
      ),
      child: Column(
        children: [
          Container(
            width: double.infinity,
            padding: const EdgeInsets.fromLTRB(4, 0, 4, 8),
            child: Text(
              _status,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(
                color: palette.textMuted,
                fontWeight: FontWeight.w600,
                fontSize: 11,
              ),
            ),
          ),
          if (_commandSuggestionsVisible) _buildCommandSuggestions(),
          Row(
            crossAxisAlignment: CrossAxisAlignment.center,
            children: [
              _buildComposerIconButton(
                icon: _voiceInputMode
                    ? Icons.keyboard_alt_rounded
                    : Icons.graphic_eq_rounded,
                onPressed: canInteract ? _toggleVoiceInputMode : null,
                active: _voiceInputMode,
                activeColor: palette.accent,
              ),
              const SizedBox(width: 10),
              Expanded(
                child: AnimatedSwitcher(
                  duration: const Duration(milliseconds: 180),
                  switchInCurve: Curves.easeOutCubic,
                  switchOutCurve: Curves.easeInCubic,
                  child: _voiceInputMode
                      ? GestureDetector(
                          key: const ValueKey('voice_composer'),
                          onLongPressStart: _handleVoiceStart,
                          onLongPressMoveUpdate: _handleVoiceMove,
                          onLongPressEnd: _handleVoiceEnd,
                          child: AnimatedContainer(
                            duration: const Duration(milliseconds: 160),
                            height: 42,
                            decoration: BoxDecoration(
                              color: _recording
                                  ? palette.accent
                                  : palette.surfaceSoft,
                              borderRadius: BorderRadius.circular(12),
                              border: Border.all(
                                color: _recording
                                    ? palette.accent.withValues(alpha: 0.95)
                                    : palette.borderStrong,
                              ),
                            ),
                            child: Center(
                              child: Text(
                                _recording ? '松手 发送' : '按住 说话',
                                style: TextStyle(
                                  color:
                                      (_recording
                                              ? _foregroundForColor(
                                                  palette.accent,
                                                )
                                              : palette.textPrimary)
                                          .withValues(
                                            alpha: _recording ? 1 : 0.9,
                                          ),
                                  fontSize: 16,
                                  fontWeight: FontWeight.w700,
                                  letterSpacing: 0.2,
                                ),
                              ),
                            ),
                          ),
                        )
                      : TextField(
                          key: const ValueKey('text_composer'),
                          controller: _messageController,
                          focusNode: _messageFocusNode,
                          minLines: 1,
                          maxLines: 3,
                          enabled: !_recording && !_transcribingVoice,
                          onChanged: (_) {
                            if (!mounted) {
                              return;
                            }
                            setState(() {});
                          },
                          onTap: () {
                            if (_voiceInputMode) {
                              setState(() {
                                _voiceInputMode = false;
                              });
                            }
                          },
                          onSubmitted: (_) => _sendMessage(),
                          style: TextStyle(
                            color: palette.textPrimary,
                            fontSize: 15,
                            height: 1.25,
                          ),
                          cursorColor: palette.accent,
                          decoration: InputDecoration(
                            hintText: _currentGroupId.isEmpty ? '发消息' : '发群消息',
                            hintStyle: TextStyle(
                              color: palette.textMuted,
                              fontSize: 15,
                            ),
                            filled: true,
                            fillColor: palette.surfaceSoft,
                            floatingLabelBehavior: FloatingLabelBehavior.never,
                            isDense: true,
                            contentPadding: const EdgeInsets.symmetric(
                              horizontal: 14,
                              vertical: 10,
                            ),
                            border: OutlineInputBorder(
                              borderRadius: BorderRadius.circular(12),
                              borderSide: BorderSide(
                                color: palette.borderStrong,
                              ),
                            ),
                            enabledBorder: OutlineInputBorder(
                              borderRadius: BorderRadius.circular(12),
                              borderSide: BorderSide(
                                color: palette.borderStrong,
                              ),
                            ),
                            focusedBorder: OutlineInputBorder(
                              borderRadius: BorderRadius.circular(12),
                              borderSide: BorderSide(color: palette.accent),
                            ),
                          ),
                        ),
                ),
              ),
              const SizedBox(width: 10),
              _buildComposerIconButton(
                icon: Icons.sentiment_satisfied_alt_rounded,
                onPressed: canInteract ? _focusTextComposer : null,
              ),
              const SizedBox(width: 10),
              if (showSendButton)
                _buildComposerIconButton(
                  icon: Icons.arrow_upward_rounded,
                  onPressed: canInteract ? _sendMessage : null,
                  active: true,
                  activeColor: palette.accent,
                )
              else
                _buildAttachmentMenuButton(enabled: canInteract),
            ],
          ),
        ],
      ),
    );
  }

  static const List<({String label, String insert, String hint})>
  _chatCommandTemplates = [
    (label: '/cg start', insert: '/cg start ', hint: '启动编码：/cg start <项目> <需求>'),
    (label: '/cg status', insert: '/cg status', hint: '查看编码会话进度'),
    (label: '/cg stop', insert: '/cg stop', hint: '停止当前编码会话'),
    (label: '/cg deploy', insert: '/cg deploy ', hint: '部署：/cg deploy <项目>'),
    (label: '/cg list', insert: '/cg list', hint: '列出可用项目'),
    (label: '/help', insert: '/help', hint: '查看全部命令'),
  ];

  bool get _commandSuggestionsVisible {
    if (_voiceInputMode) {
      return false;
    }
    final text = _messageController.text.trimLeft();
    return text.startsWith('/') && _filteredCommandTemplates(text).isNotEmpty;
  }

  List<({String label, String insert, String hint})> _filteredCommandTemplates(
    String input,
  ) {
    return _chatCommandTemplates
        .where(
          (cmd) =>
              cmd.insert.startsWith(input) ||
              (input.length > 1 && cmd.label.startsWith(input)),
        )
        .toList();
  }

  Widget _buildCommandSuggestions() {
    final palette = _palette;
    final input = _messageController.text.trimLeft();
    final commands = _filteredCommandTemplates(input);
    return Container(
      width: double.infinity,
      margin: const EdgeInsets.only(bottom: 8),
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 6),
      decoration: BoxDecoration(
        color: palette.surfaceSoft,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: palette.border),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          for (final cmd in commands)
            InkWell(
              borderRadius: BorderRadius.circular(8),
              onTap: () {
                _messageController.text = cmd.insert;
                _messageController.selection = TextSelection.collapsed(
                  offset: cmd.insert.length,
                );
                _messageFocusNode.requestFocus();
                setState(() {});
              },
              child: Padding(
                padding: const EdgeInsets.symmetric(
                  horizontal: 6,
                  vertical: 6,
                ),
                child: Row(
                  children: [
                    Text(
                      cmd.label,
                      style: TextStyle(
                        color: palette.accent,
                        fontWeight: FontWeight.w700,
                        fontSize: 13,
                      ),
                    ),
                    const SizedBox(width: 10),
                    Expanded(
                      child: Text(
                        cmd.hint,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: TextStyle(
                          color: palette.textMuted,
                          fontSize: 12,
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ),
        ],
      ),
    );
  }

  Widget _buildChatAgentCard() {
    final palette = _palette;
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 12),
      decoration: BoxDecoration(
        color: palette.surfaceMuted.withValues(alpha: 0.5),
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: palette.border.withValues(alpha: 0.55)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            '对话 Agent',
            style: TextStyle(
              fontWeight: FontWeight.w600,
              color: palette.textPrimary,
            ),
          ),
          const SizedBox(height: 4),
          Text(
            '聊天消息的默认处理方：LLM Agent（工具中枢）或 Hermes Agent（自主推理 + 博客工具）',
            style: TextStyle(fontSize: 12, color: palette.textMuted),
          ),
          const SizedBox(height: 10),
          SegmentedButton<String>(
            segments: const [
              ButtonSegment(
                value: 'llm',
                label: Text('LLM'),
                icon: Icon(Icons.hub_outlined),
              ),
              ButtonSegment(
                value: 'hermes',
                label: Text('Hermes'),
                icon: Icon(Icons.auto_awesome_outlined),
              ),
            ],
            selected: {_preferredChatAgent},
            onSelectionChanged: _chatAgentSyncing
                ? null
                : (selection) {
                    if (selection.isNotEmpty) {
                      unawaited(_applyPreferredChatAgent(selection.first));
                    }
                  },
          ),
        ],
      ),
    );
  }

  Future<void> _restorePreferredChatAgent() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      final stored = prefs.getString(_preferredChatAgentKey)?.trim() ?? '';
      if (stored == 'llm' || stored == 'hermes') {
        if (!mounted) {
          _preferredChatAgent = stored;
          return;
        }
        setState(() => _preferredChatAgent = stored);
      }
    } catch (_) {}
  }

  Future<void> _applyPreferredChatAgent(String value) async {
    final normalized = value == 'hermes' ? 'hermes' : 'llm';
    setState(() {
      _preferredChatAgent = normalized;
      _chatAgentSyncing = true;
    });
    try {
      final prefs = await SharedPreferences.getInstance();
      await prefs.setString(_preferredChatAgentKey, normalized);
      await _runAuthed('Save chat agent preference', (client) {
        return client.saveAppPreferences(<String, dynamic>{
          'preferred_chat_agent': normalized,
        });
      });
      _appendSystem(
        normalized == 'hermes' ? '对话已切换至 Hermes Agent' : '对话已切换至 LLM Agent',
        persist: false,
      );
    } catch (err) {
      _appendSystem(
        _describeRequestError(err, operation: 'Save chat agent preference'),
      );
    } finally {
      if (mounted) {
        setState(() => _chatAgentSyncing = false);
      } else {
        _chatAgentSyncing = false;
      }
    }
  }

  Future<void> _refreshChatAgentPreferenceFromServer() async {
    if (_sessionToken.isEmpty) {
      return;
    }
    try {
      final data = await _runAuthed('Load chat agent preference', (client) {
        return client.fetchAppPreferences();
      });
      final preferences = data['preferences'];
      if (preferences is! Map<String, dynamic>) {
        return;
      }
      final remote =
          (preferences['preferred_chat_agent'] ?? '').toString().trim();
      if (remote != 'llm' && remote != 'hermes') {
        return;
      }
      if (remote == _preferredChatAgent) {
        return;
      }
      final prefs = await SharedPreferences.getInstance();
      await prefs.setString(_preferredChatAgentKey, remote);
      if (!mounted) {
        _preferredChatAgent = remote;
        return;
      }
      setState(() => _preferredChatAgent = remote);
    } catch (_) {}
  }

  Widget _buildButlerBody() {
    final palette = _palette;
    return ButlerBody(
      palette: ButlerBodyPalette(
        backgroundTop: palette.backgroundTop,
        backgroundBottom: palette.backgroundBottom,
        surface: palette.surface,
        surfaceSoft: palette.surfaceSoft,
        border: palette.border,
        accent: palette.accent,
        textPrimary: palette.textPrimary,
        textSecondary: palette.textSecondary,
        textMuted: palette.textMuted,
        success: palette.success,
        error: palette.error,
      ),
      loading: _butlerLoading,
      errorText: _butlerError,
      affinityScore: _butlerAffinityScore,
      goals: _butlerGoals,
      checkpoint: _butlerCheckpoint,
      reminders: _butlerReminders,
      memoryFiles: _butlerMemoryFiles,
      selectedMemoryFile: _butlerSelectedMemoryFile,
      memoryEditorController: _butlerMemoryController,
      memoryDirty: _butlerMemoryDirty,
      onRefresh: () => unawaited(_loadButlerData()),
      onOpenMemoryFile: (file) => unawaited(_openButlerMemoryFile(file)),
      onSaveMemoryFile: () => unawaited(_saveButlerMemoryFile()),
      onMemoryChanged: () {
        if (!_butlerMemoryDirty) {
          setState(() => _butlerMemoryDirty = true);
        }
      },
      onFeedback: (kind) => unawaited(_sendButlerFeedback(kind)),
    );
  }

  List<String> _parseButlerReminders(String journal) {
    final out = <String>[];
    for (final raw in journal.split('\n')) {
      final line = raw.trim();
      if (line.isEmpty) continue;
      if (!line.contains('播报') && !line.contains('提醒')) continue;
      if (line.contains('播报反馈')) continue;
      out.add(line.replaceFirst(RegExp(r'^[-*]\s*'), ''));
    }
    if (out.length > 12) {
      return out.sublist(out.length - 12);
    }
    return out;
  }

  Future<void> _sendButlerFeedback(String kind) async {
    try {
      final resp = await _runAuthed('Butler feedback', (client) {
        return client.callButlerFeedback(kind);
      });
      final affinity = resp['affinity'];
      if (mounted && affinity is Map<String, dynamic>) {
        final score = (affinity['score'] as num?)?.toInt();
        if (score != null) {
          setState(() => _butlerAffinityScore = score);
        }
      }
      _appendSystem('已记录反馈，管家会据此调整。');
    } catch (err) {
      _appendSystem(_describeRequestError(err, operation: 'Butler feedback'));
    }
  }

  Future<void> _loadButlerData() async {
    if (_butlerLoading) {
      return;
    }
    setState(() {
      _butlerLoading = true;
      _butlerError = '';
    });
    try {
      final goalsResp = await _runAuthed('Load butler goals', (client) {
        return client.callButlerTool('RawGetCurrentGoals');
      });
      final checkpointResp = await _runAuthed('Load butler checkpoint', (
        client,
      ) {
        return client.callButlerTool(
          'RawMemoryRead',
          arguments: {'file': 'checkpoint'},
        );
      });
      final filesResp = await _runAuthed('List butler memory files', (client) {
        return client.callButlerTool('RawMemoryListFiles');
      });

      var affinityScore = _butlerAffinityScore;
      try {
        final affinityResp = await _runAuthed('Load butler affinity', (client) {
          return client.fetchButlerAffinity();
        });
        final affinity = affinityResp['affinity'];
        if (affinity is Map<String, dynamic>) {
          affinityScore = (affinity['score'] as num?)?.toInt() ?? affinityScore;
        }
      } catch (_) {
        // 养成度获取失败不应阻断面板加载。
      }

      final reminders = <String>[];
      try {
        final now = DateTime.now();
        final journalFile =
            'journal_${now.year.toString().padLeft(4, '0')}-'
            '${now.month.toString().padLeft(2, '0')}-'
            '${now.day.toString().padLeft(2, '0')}';
        final journalResp = await _runAuthed('Load butler reminders', (client) {
          return client.callButlerTool(
            'RawMemoryRead',
            arguments: {'file': journalFile},
          );
        });
        final journalResult = journalResp['result'];
        if (journalResult is Map<String, dynamic>) {
          reminders.addAll(_parseButlerReminders(
            (journalResult['content'] ?? '').toString(),
          ));
        }
      } catch (_) {
        // 提醒记录获取失败不应阻断面板加载。
      }

      final goals = <ButlerGoalSummary>[];
      final goalsResult = goalsResp['result'];
      if (goalsResult is Map<String, dynamic>) {
        for (final level in const ['daily', 'weekly', 'monthly', 'yearly']) {
          final item = goalsResult[level];
          if (item is Map<String, dynamic>) {
            goals.add(
              ButlerGoalSummary(
                level: (item['level'] ?? level).toString(),
                period: (item['period'] ?? '').toString(),
                overview: (item['overview'] ?? '').toString(),
                judge: (item['judge'] ?? '').toString(),
                progress: (item['progress'] as num?)?.toInt() ?? 0,
                status: (item['status'] ?? '').toString(),
                taskCount: (item['total_tasks'] as num?)?.toInt() ?? 0,
              ),
            );
          }
        }
      }

      var checkpoint = '';
      final checkpointResult = checkpointResp['result'];
      if (checkpointResult is Map<String, dynamic>) {
        checkpoint = (checkpointResult['content'] ?? '').toString();
      }

      final files = <String>[];
      final filesResult = filesResp['result'];
      if (filesResult is List) {
        files.addAll(filesResult.map((item) => item.toString()));
      }

      if (!mounted) {
        return;
      }
      setState(() {
        _butlerGoals = goals;
        _butlerCheckpoint = checkpoint;
        _butlerReminders = reminders;
        _butlerMemoryFiles = files;
        _butlerAffinityScore = affinityScore;
        _butlerLoaded = true;
        if (_butlerSelectedMemoryFile.isEmpty && files.isNotEmpty) {
          _butlerSelectedMemoryFile = files.contains('MEMORY')
              ? 'MEMORY'
              : files.first;
        }
      });
      if (_butlerSelectedMemoryFile.isNotEmpty && !_butlerMemoryDirty) {
        await _openButlerMemoryFile(_butlerSelectedMemoryFile);
      }
    } catch (err) {
      if (mounted) {
        setState(() {
          _butlerError = _describeRequestError(
            err,
            operation: 'Load butler data',
          );
        });
      }
    } finally {
      if (mounted) {
        setState(() => _butlerLoading = false);
      }
    }
  }

  Future<void> _openButlerMemoryFile(String file) async {
    try {
      final resp = await _runAuthed('Read butler memory', (client) {
        return client.callButlerTool('RawMemoryRead', arguments: {'file': file});
      });
      var content = '';
      final result = resp['result'];
      if (result is Map<String, dynamic>) {
        content = (result['content'] ?? '').toString();
      }
      if (!mounted) {
        return;
      }
      setState(() {
        _butlerSelectedMemoryFile = file;
        _butlerMemoryController.text = content;
        _butlerMemoryDirty = false;
      });
    } catch (err) {
      _appendSystem(_describeRequestError(err, operation: 'Read butler memory'));
    }
  }

  Future<void> _saveButlerMemoryFile() async {
    final file = _butlerSelectedMemoryFile;
    if (file.isEmpty) {
      return;
    }
    try {
      await _runAuthed('Save butler memory', (client) {
        return client.callButlerTool(
          'RawMemoryWrite',
          arguments: {'file': file, 'content': _butlerMemoryController.text},
        );
      });
      if (mounted) {
        setState(() => _butlerMemoryDirty = false);
        _appendSystem('记忆文件 $file 已保存', persist: false);
      }
    } catch (err) {
      _appendSystem(_describeRequestError(err, operation: 'Save butler memory'));
    }
  }

  Widget _buildVoiceGestureOverlay() {
    final palette = _palette;
    final activeAction = _currentVoiceGestureAction;
    final showDraft = _speechDraft.trim().isNotEmpty;

    return IgnorePointer(
      ignoring: true,
      child: Stack(
        children: [
          Positioned.fill(
            child: DecoratedBox(
              decoration: BoxDecoration(
                color: Colors.black.withValues(alpha: 0.52),
              ),
            ),
          ),
          Positioned.fill(
            child: SafeArea(
              child: Column(
                children: [
                  const Spacer(flex: 3),
                  Column(
                    children: [
                      Stack(
                        clipBehavior: Clip.none,
                        alignment: Alignment.center,
                        children: [
                          Container(
                            width: 142,
                            padding: const EdgeInsets.symmetric(
                              horizontal: 18,
                              vertical: 18,
                            ),
                            decoration: BoxDecoration(
                              color: palette.accent,
                              borderRadius: BorderRadius.circular(24),
                              boxShadow: [
                                BoxShadow(
                                  blurRadius: 32,
                                  color: palette.accent.withValues(alpha: 0.28),
                                  offset: const Offset(0, 16),
                                ),
                              ],
                            ),
                            child: const _VoiceWaveformBadge(),
                          ),
                          Positioned(
                            bottom: -8,
                            child: Transform.rotate(
                              angle: 0.78,
                              child: Container(
                                width: 16,
                                height: 16,
                                decoration: BoxDecoration(
                                  color: palette.accent,
                                  borderRadius: const BorderRadius.all(
                                    Radius.circular(4),
                                  ),
                                ),
                              ),
                            ),
                          ),
                        ],
                      ),
                      const SizedBox(height: 22),
                      Text(
                        showDraft ? _speechDraft.trim() : '正在聆听你的语音…',
                        maxLines: 2,
                        overflow: TextOverflow.ellipsis,
                        textAlign: TextAlign.center,
                        style: const TextStyle(
                          color: Colors.white,
                          fontSize: 15,
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                    ],
                  ),
                  const Spacer(flex: 2),
                  SizedBox(
                    height: 246,
                    child: Stack(
                      clipBehavior: Clip.none,
                      children: [
                        Positioned(
                          left: -58,
                          right: -58,
                          bottom: -156,
                          child: Container(
                            height: 320,
                            decoration: BoxDecoration(
                              shape: BoxShape.circle,
                              gradient: LinearGradient(
                                colors: [
                                  _blendWithAccent(
                                    palette.surfaceRaised,
                                    palette.accent,
                                    0.20,
                                  ),
                                  _blendWithAccent(
                                    palette.surface,
                                    palette.accent,
                                    0.08,
                                  ),
                                ],
                                begin: Alignment.topCenter,
                                end: Alignment.bottomCenter,
                              ),
                              border: Border.all(
                                color: palette.borderStrong.withValues(
                                  alpha: 0.7,
                                ),
                              ),
                              boxShadow: [
                                BoxShadow(
                                  blurRadius: 28,
                                  color: palette.accent.withValues(alpha: 0.16),
                                  offset: const Offset(0, -12),
                                ),
                              ],
                            ),
                            child: Padding(
                              padding: const EdgeInsets.only(top: 38),
                              child: Column(
                                children: [
                                  Text(
                                    activeAction == VoiceGestureAction.sendAudio
                                        ? '松手 发送'
                                        : activeAction ==
                                              VoiceGestureAction.cancel
                                        ? '松手 取消'
                                        : '松手 转文字',
                                    style: TextStyle(
                                      color: palette.textPrimary,
                                      fontSize: 28,
                                      fontWeight: FontWeight.w700,
                                    ),
                                  ),
                                  const SizedBox(height: 6),
                                  Text(
                                    '向左右滑动可切换操作',
                                    style: TextStyle(
                                      color: palette.textSecondary,
                                      fontSize: 13,
                                      fontWeight: FontWeight.w500,
                                    ),
                                  ),
                                ],
                              ),
                            ),
                          ),
                        ),
                        Positioned(
                          left: 14,
                          bottom: 74,
                          child: Transform.rotate(
                            angle: -0.16,
                            child: _VoiceActionPill(
                              label: '取消',
                              active: activeAction == VoiceGestureAction.cancel,
                              alignment: Alignment.centerLeft,
                            ),
                          ),
                        ),
                        Positioned(
                          right: 14,
                          bottom: 74,
                          child: Transform.rotate(
                            angle: 0.16,
                            child: _VoiceActionPill(
                              label: '滑到这里 转文字',
                              active:
                                  activeAction == VoiceGestureAction.transcribe,
                              alignment: Alignment.centerRight,
                            ),
                          ),
                        ),
                      ],
                    ),
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildChatBody() {
    final palette = _palette;
    return Stack(
      children: [
        Container(
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
            child: Column(
              children: [
                Expanded(
                  child: Padding(
                    padding: const EdgeInsets.fromLTRB(10, 10, 10, 8),
                    child: Container(
                      decoration: BoxDecoration(
                        color: palette.surface.withValues(alpha: 0.96),
                        borderRadius: BorderRadius.circular(24),
                        border: Border.all(color: palette.border),
                        boxShadow: [
                          BoxShadow(
                            blurRadius: 18,
                            color: Colors.black.withValues(alpha: 0.24),
                            offset: const Offset(0, 10),
                          ),
                        ],
                      ),
                      child: ClipRRect(
                        borderRadius: BorderRadius.circular(24),
                        child: ValueListenableBuilder<List<ChatMessage>>(
                          valueListenable: _activeMessagesNotifier,
                          builder: (context, messages, _) {
                            return ListView.builder(
                              controller: _scrollController,
                              padding: const EdgeInsets.fromLTRB(
                                14,
                                14,
                                14,
                                14,
                              ),
                              cacheExtent: 900,
                              addAutomaticKeepAlives: false,
                              addSemanticIndexes: false,
                              itemCount: messages.length,
                              itemBuilder: (context, index) {
                                final msg = messages[index];
                                return KeyedSubtree(
                                  key: _messageRowKey(msg),
                                  child: _MessageBubble(
                                    message: msg,
                                    isPlaying:
                                        _playingAudioKey ==
                                        _messagePlaybackKey(msg),
                                    onTap: () => _handleMessageTap(msg),
                                    onCopy: () async {
                                      await Clipboard.setData(
                                        ClipboardData(text: msg.content),
                                      );
                                      if (!context.mounted) {
                                        return;
                                      }
                                      ScaffoldMessenger.of(
                                        context,
                                      ).showSnackBar(
                                        const SnackBar(
                                          content: Text('Message copied'),
                                          duration: Duration(seconds: 1),
                                        ),
                                      );
                                    },
                                  ),
                                );
                              },
                            );
                          },
                        ),
                      ),
                    ),
                  ),
                ),
                _buildComposer(),
              ],
            ),
          ),
        ),
        if (_recording) Positioned.fill(child: _buildVoiceGestureOverlay()),
      ],
    );
  }

  Widget _buildCodegenBody() {
    final palette = _palette;
    return CodegenBody(
      palette: CodegenBodyPalette(
        backgroundTop: palette.backgroundTop,
        surfaceMuted: palette.surfaceMuted,
        backgroundBottom: palette.backgroundBottom,
        surface: palette.surface,
        border: palette.border,
        textPrimary: palette.textPrimary,
        textSecondary: palette.textSecondary,
        error: palette.error,
      ),
      loading: _codegenLoading,
      sending: _codegenSending,
      errorText: _codegenError,
      mode: _codegenMode,
      codingProjects: _filteredCodingProjects,
      deployProjects: _filteredDeployProjects,
      selectedCodingProject: _selectedCodingProject,
      selectedDeployProject: _selectedDeployProject,
      selectedCodeTool: _selectedCodeTool,
      selectedToolSettings: _selectedClaudeSettings,
      resumeLastSession: _codegenResumeLastSession,
      resumeLastSessionEnabled: _selectedCodeToolSupportsResume,
      selectedCodeToolOptions: _selectedCodingProjectTools,
      selectedToolSettingsOptions: _selectedToolSettingsOptions,
      selectedDeployTarget: _selectedDeployTarget,
      commandPreview: _buildCodegenCommandPreview(),
      autoDeploy: _codegenAutoDeploy,
      deployPackOnly: _deployPackOnly,
      debugBundleMode: _codegenDebugBundleMode,
      runningCodeCount: _runningCodegenCount(CodegenLaunchMode.code),
      runningDeployCount: _runningCodegenCount(CodegenLaunchMode.deploy),
      codegenPromptController: _codegenPromptController,
      codegenCodeSearchController: _codegenCodeSearchController,
      codegenDeploySearchController: _codegenDeploySearchController,
      deployArgsController: _deployArgsController,
      history: _codegenHistory,
      onRefresh: () => unawaited(_loadCodegenProjects()),
      onModeChanged: _setCodegenMode,
      onSearchChanged: (_) {
        setState(() {
          _syncFilteredCodegenSelections();
        });
        unawaited(_persistCodegenPreferences());
      },
      onCodeProjectChanged: _handleCodeProjectChanged,
      onCodeToolChanged: _handleCodeToolChanged,
      onToolSettingsChanged: _handleClaudeSettingsChanged,
      onResumeLastSessionChanged: _handleResumeLastSessionChanged,
      onPromptChanged: (_) => setState(() {}),
      onAutoDeployChanged: (value) {
        setState(() {
          _codegenAutoDeploy = value;
        });
        unawaited(_persistCodegenPreferences());
      },
      onDebugBundleModeChanged: (value) {
        setState(() {
          _codegenDebugBundleMode = value;
          if (value) {
            _codegenAutoDeploy = false;
          }
        });
        unawaited(_persistCodegenPreferences());
      },
      onDeployProjectChanged: _handleDeployProjectChanged,
      onDeployTargetChanged: _handleDeployTargetChanged,
      onDeployPackOnlyChanged: (value) {
        setState(() {
          _deployPackOnly = _selectedDeployProject?.buildOnly == true
              ? true
              : value;
        });
        unawaited(_persistCodegenPreferences());
      },
      onDeployArgsChanged: (_) {
        setState(() {});
        unawaited(_persistCodegenPreferences());
      },
      onSend: _sendCodegenCommand,
      onCommitAndPush: () => unawaited(_sendCodegenCommitCommand()),
      onSessionStatus: () => unawaited(_sendCodegenControlCommand('status')),
      onSessionStop: () => unawaited(_sendCodegenControlCommand('stop')),
      onBackupHistory: (type) => unawaited(_backupCodegenHistory(type)),
      onLoadHistoryBackup: () => unawaited(_loadCodegenHistoryBackupFromObs()),
      onClearHistory: () {
        setState(() {
          _codegenHistory.removeWhere((item) => !item.locked);
        });
        _publishCodegenHistory();
        unawaited(_persistCodegenPreferences());
      },
      onShowHistoryDetails: (item) =>
          unawaited(_showCodegenHistoryDetails(item)),
      onReExecute: (item) => _reExecuteCodegenHistory(item),
      onToggleLock: (item) => _toggleCodegenHistoryLock(item),
    );
  }

  Widget _buildSettingsBody() {
    final palette = _palette;
    final baseUrl = _clientConfig?.baseUrl ?? '';
    final receiveToken = _clientConfig?.receiveToken ?? '';

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
        child: SingleChildScrollView(
          padding: const EdgeInsets.fromLTRB(10, 10, 10, 10),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _buildConfigItem(
                icon: Icons.link_rounded,
                label: 'Server URL',
                value: baseUrl,
                onCopy: baseUrl.isEmpty
                    ? null
                    : () => unawaited(_copyText('URL', baseUrl)),
              ),
              const SizedBox(height: 8),
              _buildConfigItem(
                icon: Icons.key_rounded,
                label: 'Receive Token',
                value: receiveToken,
                onCopy: receiveToken.isEmpty
                    ? null
                    : () => unawaited(_copyText('Token', receiveToken)),
              ),
              const SizedBox(height: 12),
              Row(
                children: [
                  Expanded(
                    child: TextField(
                      controller: _baseUrlController,
                      keyboardType: TextInputType.url,
                      decoration: const InputDecoration(
                        labelText: 'Server URL',
                        hintText: 'http://127.0.0.1:9002',
                        prefixIcon: Icon(Icons.link_rounded),
                        isDense: true,
                      ),
                    ),
                  ),
                  const SizedBox(width: 10),
                  FilledButton.icon(
                    onPressed: _configLoading ? null : _saveBaseUrl,
                    style: FilledButton.styleFrom(
                      backgroundColor: palette.accent,
                      foregroundColor: _foregroundForColor(palette.accent),
                      minimumSize: const Size(0, 48),
                      padding: const EdgeInsets.symmetric(horizontal: 14),
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(18),
                      ),
                    ),
                    icon: const Icon(Icons.save_outlined),
                    label: const Text('Save URL'),
                  ),
                ],
              ),
              const SizedBox(height: 8),
              _buildSherpaModelCard(),
              const SizedBox(height: 8),
              _buildAppStorageManagerCard(),
              const SizedBox(height: 8),
              _buildThemePresetCard(),
              const SizedBox(height: 8),
              _buildChatAgentCard(),
              const SizedBox(height: 8),
              _buildCortanaChatSettings(),
            ],
          ),
        ),
      ),
    );
  }

  void _selectRootTab(RootTab nextTab) {
    _resetCortanaImmersiveUi();
    setState(() {
      _rootTab = nextTab;
      _sidebarExpanded = false;
    });
    if (nextTab == RootTab.cortana) {
      _reportCortanaUiEvent('cortana_tab_open', summary: '用户切换到了 Cortana 页签');
    }
    if (nextTab == RootTab.codegen) {
      _triggerCortanaContextualExpression('surprised');
      if (_codingProjects.isEmpty &&
          _deployProjects.isEmpty &&
          !_codegenLoading &&
          _sessionToken.isNotEmpty) {
        unawaited(_loadCodegenProjects());
      }
    }
    if (nextTab == RootTab.settings && _appStorageUsages.isEmpty) {
      unawaited(_refreshAppStorageUsage(silent: true));
    }
    if (nextTab == RootTab.settings) {
      unawaited(_refreshChatAgentPreferenceFromServer());
    }
    if (nextTab == RootTab.butler &&
        !_butlerLoaded &&
        !_butlerLoading &&
        _sessionToken.isNotEmpty) {
      unawaited(_loadButlerData());
    }
  }

  void _resetCortanaImmersiveUi() {
    _cortanaPageKey.currentState?.revealChromeAndDismissOverlays();
    if (!_cortanaImmersiveUiHidden) {
      return;
    }
    setState(() {
      _cortanaImmersiveUiHidden = false;
    });
  }

  Widget _buildSidebarNavButton({
    required IconData icon,
    required String label,
    required bool selected,
    required VoidCallback onPressed,
  }) {
    final palette = _palette;
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
      child: Material(
        color: selected
            ? palette.accent.withValues(alpha: 0.18)
            : Colors.transparent,
        borderRadius: BorderRadius.circular(16),
        child: InkWell(
          onTap: onPressed,
          borderRadius: BorderRadius.circular(16),
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 12),
            child: Row(
              children: [
                Icon(
                  icon,
                  color: selected ? palette.accent : palette.textPrimary,
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Text(
                    label,
                    style: TextStyle(
                      color: selected ? palette.accent : palette.textPrimary,
                      fontWeight: selected ? FontWeight.w800 : FontWeight.w600,
                    ),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildAppSidebar() {
    final palette = _palette;
    final width = math.min(
      math.max(MediaQuery.sizeOf(context).width - 24, 240.0),
      316.0,
    );
    return Container(
      width: width,
      decoration: BoxDecoration(
        gradient: LinearGradient(
          colors: <Color>[palette.surface, palette.surfaceRaised],
          begin: Alignment.topCenter,
          end: Alignment.bottomCenter,
        ),
        border: Border(right: BorderSide(color: palette.border)),
        boxShadow: [
          BoxShadow(
            blurRadius: 18,
            color: Colors.black.withValues(alpha: 0.18),
            offset: const Offset(10, 0),
          ),
        ],
      ),
      child: SafeArea(
        bottom: false,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(12, 10, 10, 6),
              child: Row(
                children: [
                  Icon(Icons.view_sidebar_outlined, color: palette.textPrimary),
                  const SizedBox(width: 10),
                  Expanded(
                    child: Text(
                      'Sidebar',
                      style: TextStyle(
                        color: palette.textPrimary,
                        fontWeight: FontWeight.w800,
                        fontSize: 16,
                      ),
                    ),
                  ),
                  IconButton(
                    tooltip: '关闭侧边栏',
                    onPressed: () {
                      setState(() => _sidebarExpanded = false);
                    },
                    icon: const Icon(Icons.close_rounded),
                  ),
                ],
              ),
            ),
            _buildSidebarNavButton(
              icon: Icons.chat_bubble_outline_rounded,
              label: '聊天',
              selected: _rootTab == RootTab.chat,
              onPressed: () => _selectRootTab(RootTab.chat),
            ),
            _buildSidebarNavButton(
              icon: Icons.terminal_outlined,
              label: '编码发布',
              selected: _rootTab == RootTab.codegen,
              onPressed: () => _selectRootTab(RootTab.codegen),
            ),
            _buildSidebarNavButton(
              icon: Icons.face_outlined,
              label: 'Cortana',
              selected: _rootTab == RootTab.cortana,
              onPressed: () => _selectRootTab(RootTab.cortana),
            ),
            _buildSidebarNavButton(
              icon: Icons.bug_report_outlined,
              label: '调试',
              selected: _rootTab == RootTab.debug,
              onPressed: () => _selectRootTab(RootTab.debug),
            ),
            _buildSidebarNavButton(
              icon: Icons.settings_outlined,
              label: '设置',
              selected: _rootTab == RootTab.settings,
              onPressed: () => _selectRootTab(RootTab.settings),
            ),
            Divider(color: palette.border, height: 18),
            if (_rootTab == RootTab.chat)
              Expanded(
                child: SingleChildScrollView(
                  padding: const EdgeInsets.fromLTRB(10, 0, 10, 16),
                  child: _buildChatControlsPanel(),
                ),
              )
            else
              Padding(
                padding: const EdgeInsets.fromLTRB(18, 8, 18, 0),
                child: Text(
                  'Direct、Groups 和 Controls 已集中到聊天侧边栏。',
                  style: TextStyle(
                    color: palette.textSecondary,
                    fontSize: 12,
                    height: 1.35,
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }

  void _triggerCortanaContextualExpression(String expression) {
    _cortanaExpressionTimer?.cancel();
    setState(() {
      _cortanaContextualExpression = expression;
    });
    _cortanaExpressionTimer = Timer(const Duration(seconds: 3), () {
      if (!mounted) return;
      setState(() {
        _cortanaContextualExpression = null;
      });
    });
  }

  Widget _buildCortanaLayer() {
    final isFullscreen = _rootTab == RootTab.cortana;
    final floatingBottomInset = _rootTab == RootTab.chat ? 116.0 : 28.0;
    return CortanaPage(
      key: _cortanaPageKey,
      mode: isFullscreen ? CortanaDisplayMode.fullscreen : _cortanaFloatingMode,
      onSendMessage: _sendCortanaMessage,
      externalVoiceHistory: _buildCortanaReplayHistory(),
      contextualExpression: _cortanaContextualExpression,
      showBadge: _cortanaBadge,
      voiceWakeListening: _cortanaWakeListening,
      awaitingVoiceCommand: _cortanaWakeAwaitingCommand,
      autoCollapseDelay: const Duration(seconds: 8),
      floatingBottomInset: floatingBottomInset,
      onTapWhenFloating: () {
        setState(() {
          _cortanaBadge = false;
          _cortanaFloatingMode =
              _cortanaFloatingMode == CortanaDisplayMode.collapsed
              ? CortanaDisplayMode.small
              : _cortanaFloatingMode;
        });
        _reportCortanaUiEvent(
          'cortana_floating_tap',
          summary: '用户点击了 Cortana 浮动按钮并展开小窗',
        );
      },
      onLongPressWhenFloating: () {
        setState(() {
          _cortanaBadge = false;
          _rootTab = RootTab.cortana;
          _cortanaFloatingMode = CortanaDisplayMode.collapsed;
        });
        _reportCortanaUiEvent(
          'cortana_floating_long_press',
          summary: '用户长按 Cortana 浮动按钮并进入页签',
        );
      },
      onModeChanged: (mode) {
        setState(() {
          _cortanaFloatingMode = mode;
          if (mode == CortanaDisplayMode.collapsed) {
            _cortanaBadge = false;
          }
        });
      },
      settings: _cortanaSettings,
      onSettingsChanged: _applyCortanaSettings,
      live2dModelUrl: _selectedCortanaLive2dModelUrl(),
      viewTransform: _selectedCortanaLive2dViewTransform(),
      onViewTransformChanged: _handleCortanaLive2dViewTransformChanged,
      onImmersiveUiChanged: (hidden) {
        if (_cortanaImmersiveUiHidden == hidden) {
          return;
        }
        setState(() {
          _cortanaImmersiveUiHidden = hidden;
        });
      },
    );
  }

  Widget _buildDebugBody() {
    final palette = _palette;
    final events = _llmDebugEvents.reversed.toList(growable: false);
    return SafeArea(
      child: Column(
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 86, 16, 10),
            child: Row(
              children: [
                Expanded(
                  child: Text(
                    'LLM 调试',
                    style: TextStyle(
                      color: palette.textPrimary,
                      fontSize: 20,
                      fontWeight: FontWeight.w800,
                    ),
                  ),
                ),
                IconButton(
                  tooltip: '复制完整调试上下文',
                  onPressed: () => unawaited(_copyFullDebugContext()),
                  icon: const Icon(Icons.integration_instructions_outlined),
                ),
                IconButton(
                  tooltip: '复制客户端日志',
                  onPressed: () => unawaited(_copyRecentFlutterClientLogs()),
                  icon: const Icon(Icons.copy_all_outlined),
                ),
                IconButton(
                  tooltip: '上报当前状态',
                  onPressed: () => unawaited(_reportDebugAppState()),
                  icon: const Icon(Icons.bug_report_outlined),
                ),
                IconButton(
                  tooltip: '清空',
                  onPressed: _llmDebugEvents.isEmpty
                      ? null
                      : () => setState(_llmDebugEvents.clear),
                  icon: const Icon(Icons.delete_sweep_outlined),
                ),
              ],
            ),
          ),
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 0, 16, 10),
            child: _buildCortanaChatLogs(),
          ),
          Expanded(
            child: events.isEmpty
                ? Center(
                    child: Text(
                      '暂无调试事件',
                      style: TextStyle(color: palette.textSecondary),
                    ),
                  )
                : ListView.separated(
                    padding: const EdgeInsets.fromLTRB(16, 0, 16, 24),
                    itemCount: events.length,
                    separatorBuilder: (_, _) => const SizedBox(height: 10),
                    itemBuilder: (context, index) =>
                        _buildDebugEventTile(events[index]),
                  ),
          ),
        ],
      ),
    );
  }

  Widget _buildDebugEventTile(LlmDebugEvent event) {
    final palette = _palette;
    final payload = event.payload;
    final summary = _debugEventSummary(event, payload);
    return Material(
      color: palette.surfaceRaised.withValues(alpha: 0.92),
      borderRadius: BorderRadius.circular(8),
      child: ExpansionTile(
        tilePadding: const EdgeInsets.symmetric(horizontal: 14, vertical: 4),
        childrenPadding: const EdgeInsets.fromLTRB(14, 0, 14, 14),
        leading: Icon(_debugEventIcon(event.event), color: palette.accent),
        title: Text(
          event.label,
          style: TextStyle(
            color: palette.textPrimary,
            fontWeight: FontWeight.w800,
          ),
        ),
        subtitle: Text(
          '${_formatTime(event.timestamp)}  $summary',
          maxLines: 2,
          overflow: TextOverflow.ellipsis,
          style: TextStyle(color: palette.textSecondary, height: 1.3),
        ),
        children: [
          Align(
            alignment: Alignment.centerLeft,
            child: SelectableText(
              event.detailText,
              style: TextStyle(
                color: palette.textPrimary,
                fontFamily: 'monospace',
                fontSize: 12,
                height: 1.35,
              ),
            ),
          ),
        ],
      ),
    );
  }

  String _debugEventSummary(LlmDebugEvent event, Map<String, dynamic> payload) {
    if (event.event == 'debug_prompt') {
      return 'system ${payload['system_prompt_chars'] ?? 0} chars · tools ${payload['visible_tools_count'] ?? 0}';
    }
    if (event.event == 'debug_llm_round') {
      return '第 ${payload['iteration'] ?? '?'} 轮 · ${payload['duration'] ?? ''} · tool_calls ${payload['tool_calls_count'] ?? 0}';
    }
    return event.content.replaceAll('\n', ' ');
  }

  IconData _debugEventIcon(String event) {
    switch (event) {
      case 'debug_prompt':
        return Icons.article_outlined;
      case 'debug_llm_round':
        return Icons.psychology_alt_outlined;
      case 'tool_call':
      case 'tool_result':
      case 'tool_progress':
        return Icons.build_circle_outlined;
      case 'task_complete':
        return Icons.check_circle_outline;
      default:
        return Icons.notes_outlined;
    }
  }

  String _formatTime(DateTime time) {
    final hh = time.hour.toString().padLeft(2, '0');
    final mm = time.minute.toString().padLeft(2, '0');
    final ss = time.second.toString().padLeft(2, '0');
    return '$hh:$mm:$ss';
  }
}
