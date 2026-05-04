import 'dart:async';
import 'dart:io';

import 'package:audioplayers/audioplayers.dart';
import 'package:flutter/material.dart';

import 'cortana_page.dart';

class CortanaHistoryPage extends StatefulWidget {
  const CortanaHistoryPage({
    super.key,
    required this.onLoadHistory,
    required this.onPreparePlayback,
    this.title = 'Cortana 播报历史',
  });

  final Future<List<CortanaReplayItem>> Function() onLoadHistory;
  final Future<CortanaReplayItem> Function(CortanaReplayItem item)
  onPreparePlayback;
  final String title;

  @override
  State<CortanaHistoryPage> createState() => _CortanaHistoryPageState();
}

class _CortanaHistoryPageState extends State<CortanaHistoryPage> {
  final AudioPlayer _audioPlayer = AudioPlayer();
  List<CortanaReplayItem> _items = const <CortanaReplayItem>[];
  bool _loading = true;
  String _error = '';
  String _playingItemId = '';

  @override
  void initState() {
    super.initState();
    unawaited(_loadHistory());
    _audioPlayer.onPlayerComplete.listen((_) {
      if (!mounted) {
        return;
      }
      setState(() {
        _playingItemId = '';
      });
    });
  }

  @override
  void dispose() {
    unawaited(_audioPlayer.dispose());
    super.dispose();
  }

  Future<void> _loadHistory() async {
    setState(() {
      _loading = true;
      _error = '';
    });
    try {
      final items = await widget.onLoadHistory();
      if (!mounted) {
        return;
      }
      setState(() {
        _items = items;
        _loading = false;
      });
    } catch (err) {
      if (!mounted) {
        return;
      }
      setState(() {
        _loading = false;
        _error = err.toString();
      });
    }
  }

  Future<void> _togglePlayback(CortanaReplayItem item) async {
    if (_playingItemId == item.id) {
      await _audioPlayer.stop();
      if (!mounted) {
        return;
      }
      setState(() {
        _playingItemId = '';
      });
      return;
    }

    final prepared = await widget.onPreparePlayback(item);
    if (prepared.audioPath.trim().isNotEmpty) {
      final audioFile = File(prepared.audioPath);
      if (!await audioFile.exists()) {
        throw Exception('语音文件不存在: ${prepared.audioPath}');
      }
      await _audioPlayer.stop();
      await _audioPlayer.play(DeviceFileSource(prepared.audioPath));
    } else if (prepared.audioBytes != null) {
      await _audioPlayer.stop();
      await _audioPlayer.play(BytesSource(prepared.audioBytes!));
    } else {
      throw const FormatException('历史播报缺少可播放音频');
    }

    if (!mounted) {
      return;
    }
    setState(() {
      _playingItemId = prepared.id;
    });
  }

  String _formatCreatedAt(DateTime time) {
    final year = time.year.toString().padLeft(4, '0');
    final month = time.month.toString().padLeft(2, '0');
    final day = time.day.toString().padLeft(2, '0');
    final hour = time.hour.toString().padLeft(2, '0');
    final minute = time.minute.toString().padLeft(2, '0');
    final second = time.second.toString().padLeft(2, '0');
    return '$year-$month-$day $hour:$minute:$second';
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: Text(widget.title),
        actions: [
          IconButton(
            tooltip: '刷新',
            onPressed: _loading ? null : _loadHistory,
            icon: const Icon(Icons.refresh_rounded),
          ),
        ],
      ),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _error.trim().isNotEmpty
          ? Center(child: Text(_error))
          : _items.isEmpty
          ? const Center(child: Text('暂无 Cortana 播报历史'))
          : ListView.separated(
              itemCount: _items.length,
              separatorBuilder: (_, _) => const Divider(height: 1),
              itemBuilder: (context, index) {
                final item = _items[index];
                final isPlaying = _playingItemId == item.id;
                return ListTile(
                  leading: Icon(
                    isPlaying
                        ? Icons.pause_circle_filled_rounded
                        : Icons.play_circle_fill_rounded,
                  ),
                  title: Text(
                    item.text,
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                  ),
                  subtitle: Text(
                    '${_formatCreatedAt(item.createdAt)}'
                    '${item.audioFormat.trim().isEmpty ? '' : ' · ${item.audioFormat}'}'
                    '${item.sourceLabel.trim().isEmpty ? '' : ' · ${item.sourceLabel}'}',
                  ),
                  trailing: IconButton(
                    tooltip: isPlaying ? '停止' : '播放',
                    onPressed: () => _togglePlayback(item),
                    icon: Icon(
                      isPlaying
                          ? Icons.stop_circle_outlined
                          : Icons.play_arrow_rounded,
                    ),
                  ),
                );
              },
            ),
    );
  }
}
