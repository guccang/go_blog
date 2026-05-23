// ignore_for_file: invalid_use_of_protected_member
part of '../../main.dart';

extension _ChatPageStateVoiceAttachments on _ChatPageState {
  bool get _composerHasText => _messageController.text.trim().isNotEmpty;

  void _toggleVoiceInputMode() {
    if (_recording || _sending || _transcribingVoice) {
      return;
    }
    final nextMode = !_voiceInputMode;
    setState(() {
      _voiceInputMode = nextMode;
    });
    if (nextMode) {
      FocusScope.of(context).unfocus();
      return;
    }
    _messageFocusNode.requestFocus();
  }

  void _focusTextComposer() {
    if (_recording || _transcribingVoice) {
      return;
    }
    if (_voiceInputMode) {
      setState(() {
        _voiceInputMode = false;
      });
    }
    _messageFocusNode.requestFocus();
  }

  VoiceGestureAction get _currentVoiceGestureAction =>
      resolveVoiceGestureAction(_recordDragOffset);

  Offset _resolveVoiceDragOffset({
    Offset? globalPosition,
    Offset? fallbackOffset,
  }) {
    final origin = _recordDragStartGlobalPosition;
    if (origin != null && globalPosition != null) {
      return globalPosition - origin;
    }
    return fallbackOffset ?? _recordDragOffset;
  }

  Future<void> _handleVoiceStart(LongPressStartDetails details) async {
    if (_sessionToken.isEmpty && _refreshToken.isEmpty) {
      _appendSystem('Please login first.');
      return;
    }
    if (_recording || _sending) {
      return;
    }
    await _pauseCortanaWakeListening(cancel: true);

    final hasPermission = await _audioRecorder.hasPermission();
    if (!hasPermission) {
      _appendSystem('Microphone permission denied.');
      _scheduleCortanaWakeRestart();
      return;
    }

    try {
      final tempDir = await getTemporaryDirectory();
      final useWaveFile = _isWindowsHost || _useLocalVosk;
      final fileExt = useWaveFile ? 'wav' : 'm4a';
      final path =
          '${tempDir.path}${Platform.pathSeparator}app_voice_${DateTime.now().millisecondsSinceEpoch}.$fileExt';
      await _audioRecorder.start(
        RecordConfig(
          encoder: useWaveFile ? AudioEncoder.wav : AudioEncoder.aacLc,
          bitRate: useWaveFile ? 256000 : 64000,
          sampleRate: useWaveFile ? 16000 : 16000,
          numChannels: 1,
        ),
        path: path,
      );
      if (_speechReady && !_useLocalVosk) {
        try {
          // Check if locale is available
          final locales = await _speechToText.locales();
          final zhLocale = locales.firstWhere(
            (l) => l.localeId == 'zh_CN' || l.localeId.startsWith('zh'),
            orElse: () => locales.first,
          );
          _appendSystem(
            'Using speech locale: ${zhLocale.name} (${zhLocale.localeId})',
          );
        } catch (e) {
          _appendSystem('Get locales failed: $e');
        }

        try {
          await _speechToText.listen(
            onResult: (result) {
              if (!mounted) {
                return;
              }
              final words = normalizeSpeechTranscript(result.recognizedWords);
              if (words.isNotEmpty) {
                _appendSystem('Recognized: $words');
              }
              setState(() {
                _speechDraft = words;
              });
            },
            onSoundLevelChange: (level) {
              // Sound level changes - useful for debugging
              if (!mounted) return;
              debugPrint('Sound level: $level');
            },
            pauseFor: const Duration(seconds: 2),
            listenFor: const Duration(minutes: 1),
            localeId: 'zh_CN',
            listenOptions: stt.SpeechListenOptions(
              listenMode: stt.ListenMode.dictation,
              partialResults: true,
              cancelOnError: false,
            ),
          );
          _appendSystem('Speech listen started: ${_speechToText.isListening}');
        } catch (e, stack) {
          _appendSystem('Speech listen failed: $e');
          debugPrint('Speech listen error: $e\n$stack');
        }
      }
      if (!mounted) {
        return;
      }
      setState(() {
        _recording = true;
        _recordDragOffset = Offset.zero;
        _recordDragStartGlobalPosition = details.globalPosition;
        _speechDraft = '';
        _recordStartedAt = DateTime.now();
        _status = 'Recording...';
      });
    } catch (err) {
      _appendSystem('Voice record start failed: $err');
      _scheduleCortanaWakeRestart();
    }
  }

  void _handleVoiceMove(LongPressMoveUpdateDetails details) {
    if (!_recording) {
      return;
    }
    setState(() {
      _recordDragOffset = _resolveVoiceDragOffset(
        globalPosition: details.globalPosition,
        fallbackOffset: details.offsetFromOrigin,
      );
    });
  }

  Future<void> _handleVoiceEnd(LongPressEndDetails details) async {
    if (!_recording) {
      return;
    }
    final dragOffset = _resolveVoiceDragOffset(
      globalPosition: details.globalPosition,
      fallbackOffset: _recordDragOffset,
    );
    if (mounted) {
      setState(() {
        _recordDragOffset = dragOffset;
      });
    } else {
      _recordDragOffset = dragOffset;
    }

    switch (resolveVoiceGestureAction(dragOffset)) {
      case VoiceGestureAction.cancel:
        await _cancelVoice();
        return;
      case VoiceGestureAction.transcribe:
        await _transcribeVoiceToDraft();
        return;
      case VoiceGestureAction.sendAudio:
        await _sendVoiceAsAudio();
    }
  }

  Future<RecordedAudio?> _stopRecording({required bool discard}) async {
    final startedAt = _recordStartedAt;
    _recordStartedAt = null;
    _recordDragStartGlobalPosition = null;
    await _stopSpeechRecognition(cancel: false);

    String? path;
    try {
      path = await _audioRecorder.stop();
    } catch (_) {}

    final duration = startedAt == null
        ? Duration.zero
        : DateTime.now().difference(startedAt);

    if (!mounted) {
      return null;
    }
    setState(() {
      _recording = false;
      _recordDragOffset = Offset.zero;
    });

    if (path == null || path.isEmpty) {
      return null;
    }
    if (discard) {
      try {
        await File(path).delete();
      } catch (_) {}
      return null;
    }
    return RecordedAudio(path: path, duration: duration);
  }

  Future<void> _cancelVoice() async {
    await _stopSpeechRecognition(cancel: true);
    await _stopRecording(discard: true);
    _appendSystem('Voice input cancelled.');
    _scheduleCortanaWakeRestart();
  }

  Future<void> _transcribeVoiceToDraft() async {
    final recorded = await _stopRecording(discard: false);
    if (recorded == null) {
      _appendSystem('语音录制不可用。');
      return;
    }
    try {
      if (mounted) {
        setState(() {
          _transcribingVoice = true;
          _status = '语音转文字中...';
        });
      }

      var transcript = _speechDraft.trim();
      if (_useLocalVosk) {
        transcript = await _voskTranscriber.transcribeFile(recorded.path);
      }
      transcript = normalizeSpeechTranscript(transcript);

      if (transcript.isEmpty) {
        _appendSystem('未识别到有效语音内容，请重试。');
        return;
      }
      final wakeCommand = _extractCortanaWakeCommand(transcript);
      if (wakeCommand != null) {
        _appendSystem('语音输入命中唤醒词，切换到 Cortana。');
        _appendCortanaWakeLog(
          '手动语音输入命中唤醒词，原始文本="$transcript", 初始指令="$wakeCommand"',
        );
        if (mounted) {
          setState(() {
            _voiceInputMode = false;
            _status = 'Cortana 已唤醒';
          });
        }
        unawaited(_handleCortanaWakeDetectedSafely(wakeCommand));
        return;
      }

      final existing = _messageController.text.trim();
      final merged = existing.isEmpty ? transcript : '$existing\n$transcript';
      _messageController.value = TextEditingValue(
        text: merged,
        selection: TextSelection.collapsed(offset: merged.length),
      );
      _speechDraft = transcript;
      if (mounted) {
        setState(() {
          _voiceInputMode = false;
          _status = '语音已转成文字，可修改后发送';
        });
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (!mounted) {
            return;
          }
          _messageFocusNode.requestFocus();
        });
      }
    } catch (err) {
      _appendSystem('本地语音识别失败：$err');
    } finally {
      try {
        await File(recorded.path).delete();
      } catch (_) {}
      if (mounted) {
        setState(() {
          _transcribingVoice = false;
        });
      }
      _scheduleCortanaWakeRestart();
    }
  }

  Future<void> _sendVoiceAsAudio() async {
    final recorded = await _stopRecording(discard: false);
    if (recorded == null) {
      _appendSystem('Voice recording unavailable.');
      _scheduleCortanaWakeRestart();
      return;
    }

    try {
      final file = File(recorded.path);
      final bytes = await file.readAsBytes();
      await file.delete();
      if (bytes.length > 768 * 1024) {
        _appendSystem('Voice message too large. Please keep it shorter.');
        return;
      }

      final seconds = recorded.duration.inMilliseconds / 1000;
      final label = '[Voice ${seconds.toStringAsFixed(1)}s]';
      final audioFormat = (_isWindowsHost || _useLocalVosk) ? 'wav' : 'm4a';
      final savedAudioPath = await _persistVoiceMessage(
        bytes: bytes,
        extension: audioFormat,
      );
      _appendOutgoing(
        label,
        messageType: 'audio',
        meta: <String, dynamic>{
          'audio_path': savedAudioPath,
          'audio_format': audioFormat,
          'duration_ms': recorded.duration.inMilliseconds,
          if (_speechDraft.trim().isNotEmpty)
            'speech_text': _speechDraft.trim(),
          'input_mode': 'voice_audio',
          if (_currentGroupId.isNotEmpty) 'group_id': _currentGroupId,
          if (_currentGroupId.isNotEmpty) 'scope': 'group',
        },
      );
      setState(() {
        _sending = true;
      });
      try {
        await _runAuthed('Send voice audio', (client) {
          return client.sendAppMessage(
            label,
            messageType: 'audio',
            meta: <String, dynamic>{
              'audio_base64': base64Encode(bytes),
              'audio_format': audioFormat,
              'duration_ms': recorded.duration.inMilliseconds,
              if (_speechDraft.trim().isNotEmpty)
                'speech_text': _speechDraft.trim(),
              'input_mode': 'voice_audio',
              if (_currentGroupId.isNotEmpty) 'group_id': _currentGroupId,
              if (_currentGroupId.isNotEmpty) 'scope': 'group',
            },
          );
        });
        if (mounted) {
          setState(() {
            _status = 'Voice audio sent';
          });
        }
      } finally {
        if (mounted) {
          setState(() {
            _sending = false;
          });
        }
      }
    } catch (err) {
      _appendSystem(_describeRequestError(err, operation: 'Send voice audio'));
    } finally {
      _scheduleCortanaWakeRestart();
    }
  }

  Future<void> _handleAttachmentMenuAction(_AttachmentMenuAction action) async {
    switch (action) {
      case _AttachmentMenuAction.galleryImage:
        return _pickAndSendImage(ImageSource.gallery);
      case _AttachmentMenuAction.cameraImage:
        return _pickAndSendImage(ImageSource.camera);
      case _AttachmentMenuAction.imageResource:
        return _pickAndUploadImageResource();
      case _AttachmentMenuAction.live2dResource:
        return _promptAndUploadResourcePath('live2d');
      case _AttachmentMenuAction.fileResource:
        return _promptAndUploadResourcePath('file');
      case _AttachmentMenuAction.browseResources:
        return _openResourceLibrary();
    }
  }

  Future<void> _pickAndUploadImageResource() async {
    if (_sessionToken.isEmpty && _refreshToken.isEmpty) {
      _appendSystem('Please login first.');
      return;
    }
    if (_sending || _recording) {
      return;
    }
    try {
      final picked = await _imagePicker.pickImage(
        source: ImageSource.gallery,
        imageQuality: 96,
      );
      if (picked == null) {
        return;
      }
      final bytes = await picked.readAsBytes();
      if (bytes.isEmpty) {
        _appendSystem('Selected image is empty.');
        return;
      }
      final fileName = picked.name.trim().isEmpty
          ? 'image_${DateTime.now().millisecondsSinceEpoch}.jpg'
          : picked.name.trim();
      setState(() {
        _sending = true;
        _status = 'Uploading image resource...';
      });
      try {
        final imageFormat = _detectImageFormat(fileName, bytes);
        final item = await _runAuthed('Upload image resource', (client) {
          return client.uploadResourceBytes(
            category: 'image',
            fileName: fileName,
            bytes: bytes,
            contentType: imageFormat == 'jpg'
                ? 'image/jpeg'
                : 'image/$imageFormat',
          );
        });
        _appendSystem('图片资源已上传：${item.fileName}');
        if (mounted) {
          setState(() => _status = 'Image resource uploaded');
        }
      } finally {
        if (mounted) {
          setState(() => _sending = false);
        }
      }
    } catch (err) {
      _appendSystem(
        _describeRequestError(err, operation: 'Upload image resource'),
      );
      if (mounted) {
        setState(() => _sending = false);
      }
    }
  }

  Future<void> _promptAndUploadResourcePath(String category) async {
    if (_sessionToken.isEmpty && _refreshToken.isEmpty) {
      _appendSystem('Please login first.');
      return;
    }
    if (!_isAndroidHost) {
      _appendSystem('当前平台暂不支持系统文件选择器，请在 Android 设备上选择文件上传。');
      return;
    }
    if (_sending || _recording) {
      return;
    }
    try {
      final path = await _localFilePicker.pickFile();
      if (path == null || path.trim().isEmpty) {
        return;
      }
      final file = File(path.trim());
      if (!await file.exists()) {
        _appendSystem('文件不存在：${path.trim()}');
        return;
      }
      final fileLength = await file.length();
      debugPrint(
        '[App Resource] upload start: category=$category path=${file.path} bytes=$fileLength',
      );
      addFlutterClientLog(
        '资源上传开始: category=$category path=${file.path} size=${_formatBytes(fileLength)}',
      );
      setState(() {
        _sending = true;
        _status = 'Uploading resource...';
      });
      try {
        final item = await _runAuthed('Upload resource', (client) {
          return client.uploadResourceFile(
            category: category,
            path: path.trim(),
          );
        });
        _appendSystem(
          '资源已上传：${item.fileName} (${_resourceCategoryLabel(item.category)})',
        );
        debugPrint(
          '[App Resource] upload done: category=${item.category} fileId=${item.fileId} name=${item.fileName} size=${item.fileSize} format=${item.fileFormat}',
        );
        addFlutterClientLog(
          '资源上传完成: ${item.fileName} fileId=${item.fileId} category=${item.category} size=${_formatBytes(item.fileSize)}',
        );
        if (mounted) {
          setState(() => _status = 'Resource uploaded');
        }
      } finally {
        if (mounted) {
          setState(() => _sending = false);
        }
      }
    } catch (err) {
      _appendSystem(_describeRequestError(err, operation: 'Upload resource'));
      if (mounted) {
        setState(() => _sending = false);
      }
    }
  }

  Future<void> _openResourceLibrary() async {
    if (_sessionToken.isEmpty && _refreshToken.isEmpty) {
      _appendSystem('Please login first.');
      return;
    }
    String selectedCategory = 'live2d';
    final deletingFileIds = <String>{};
    Future<AppResourceListResult> load(String category) {
      return _runAuthed('List resources', (client) {
        return client.listResources(category);
      });
    }

    Future<AppResourceListResult> future = load(selectedCategory);
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      backgroundColor: Colors.transparent,
      builder: (context) {
        final palette = context.appPalette;
        return StatefulBuilder(
          builder: (context, setSheetState) {
            return SafeArea(
              child: Container(
                constraints: BoxConstraints(
                  maxHeight: MediaQuery.sizeOf(context).height * 0.78,
                ),
                padding: const EdgeInsets.fromLTRB(16, 14, 16, 16),
                decoration: BoxDecoration(
                  color: palette.surface,
                  borderRadius: const BorderRadius.vertical(
                    top: Radius.circular(18),
                  ),
                  border: Border.all(color: palette.border),
                ),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        Icon(Icons.inventory_2_outlined, color: palette.accent),
                        const SizedBox(width: 8),
                        Expanded(
                          child: Text(
                            '资源库',
                            style: TextStyle(
                              color: palette.textPrimary,
                              fontSize: 18,
                              fontWeight: FontWeight.w800,
                            ),
                          ),
                        ),
                        IconButton(
                          tooltip: '刷新',
                          onPressed: () {
                            setSheetState(() {
                              future = load(selectedCategory);
                            });
                          },
                          icon: const Icon(Icons.refresh_rounded),
                        ),
                      ],
                    ),
                    const SizedBox(height: 10),
                    Wrap(
                      spacing: 8,
                      runSpacing: 8,
                      children: ['live2d', 'image', 'file'].map((category) {
                        final selected = selectedCategory == category;
                        return ChoiceChip(
                          selected: selected,
                          label: Text(_resourceCategoryLabel(category)),
                          onSelected: (_) {
                            setSheetState(() {
                              selectedCategory = category;
                              future = load(category);
                            });
                          },
                        );
                      }).toList(),
                    ),
                    const SizedBox(height: 12),
                    Expanded(
                      child: FutureBuilder<AppResourceListResult>(
                        future: future,
                        builder: (context, snapshot) {
                          if (snapshot.connectionState !=
                              ConnectionState.done) {
                            return const Center(
                              child: CircularProgressIndicator(),
                            );
                          }
                          if (snapshot.hasError) {
                            return Center(
                              child: Text(
                                _describeRequestError(
                                  snapshot.error!,
                                  operation: 'List resources',
                                ),
                                style: TextStyle(color: palette.error),
                              ),
                            );
                          }
                          final result =
                              snapshot.data ??
                              AppResourceListResult(
                                items: const <AppResourceItem>[],
                                usage: AppResourceUsage.fromJson(null),
                              );
                          final items = result.items;
                          final usage = result.usage;
                          if (items.isEmpty) {
                            return Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                _buildResourceUsageBanner(
                                  categoryLabel: _resourceCategoryLabel(
                                    selectedCategory,
                                  ),
                                  usage: usage,
                                ),
                                const SizedBox(height: 12),
                                Expanded(
                                  child: Center(
                                    child: Text(
                                      '暂无${_resourceCategoryLabel(selectedCategory)}资源',
                                      style: TextStyle(
                                        color: palette.textSecondary,
                                      ),
                                    ),
                                  ),
                                ),
                              ],
                            );
                          }
                          return Column(
                            children: [
                              _buildResourceUsageBanner(
                                categoryLabel: _resourceCategoryLabel(
                                  selectedCategory,
                                ),
                                usage: usage,
                              ),
                              const SizedBox(height: 8),
                              Expanded(
                                child: ListView.separated(
                                  itemCount: items.length,
                                  separatorBuilder: (_, _) =>
                                      Divider(height: 1, color: palette.border),
                                  itemBuilder: (context, index) {
                                    final item = items[index];
                                    final deleting = deletingFileIds.contains(
                                      item.fileId,
                                    );
                                    return ListTile(
                                      contentPadding: EdgeInsets.zero,
                                      leading: Icon(
                                        _resourceCategoryIcon(item.category),
                                        color: palette.accent,
                                      ),
                                      title: Text(
                                        item.fileName,
                                        maxLines: 1,
                                        overflow: TextOverflow.ellipsis,
                                        style: TextStyle(
                                          color: palette.textPrimary,
                                          fontWeight: FontWeight.w700,
                                        ),
                                      ),
                                      subtitle: Text(
                                        '${_formatBytes(item.fileSize)} · ${item.storageProvider.isEmpty ? 'local' : item.storageProvider}',
                                        style: TextStyle(
                                          color: palette.textSecondary,
                                        ),
                                      ),
                                      trailing: Row(
                                        mainAxisSize: MainAxisSize.min,
                                        children: [
                                          if (item.category == 'live2d')
                                            IconButton(
                                              tooltip: '安装并切换',
                                              onPressed:
                                                  _cortanaLive2dDownloading ||
                                                      deleting
                                                  ? null
                                                  : () {
                                                      unawaited(
                                                        _installCortanaLive2dResourceItem(
                                                          item,
                                                        ),
                                                      );
                                                    },
                                              icon: const Icon(
                                                Icons.face_retouching_natural,
                                              ),
                                            ),
                                          IconButton(
                                            tooltip: '下载',
                                            onPressed: deleting
                                                ? null
                                                : () {
                                                    unawaited(
                                                      _downloadResourceItem(
                                                        item,
                                                      ),
                                                    );
                                                  },
                                            icon: const Icon(
                                              Icons.download_rounded,
                                            ),
                                          ),
                                          IconButton(
                                            tooltip: '删除',
                                            onPressed: deleting
                                                ? null
                                                : () async {
                                                    final confirmed =
                                                        await _confirmDeleteResourceItem(
                                                          item,
                                                        );
                                                    if (!confirmed) {
                                                      return;
                                                    }
                                                    setSheetState(() {
                                                      deletingFileIds.add(
                                                        item.fileId,
                                                      );
                                                    });
                                                    try {
                                                      await _deleteResourceItem(
                                                        item,
                                                      );
                                                      setSheetState(() {
                                                        deletingFileIds.remove(
                                                          item.fileId,
                                                        );
                                                        future = load(
                                                          selectedCategory,
                                                        );
                                                      });
                                                    } catch (_) {
                                                      setSheetState(() {
                                                        deletingFileIds.remove(
                                                          item.fileId,
                                                        );
                                                      });
                                                    }
                                                  },
                                            icon: deleting
                                                ? const SizedBox(
                                                    width: 18,
                                                    height: 18,
                                                    child:
                                                        CircularProgressIndicator(
                                                          strokeWidth: 2,
                                                        ),
                                                  )
                                                : Icon(
                                                    Icons.delete_outline,
                                                    color: palette.error,
                                                  ),
                                          ),
                                        ],
                                      ),
                                    );
                                  },
                                ),
                              ),
                            ],
                          );
                        },
                      ),
                    ),
                  ],
                ),
              ),
            );
          },
        );
      },
    );
  }

  Widget _buildResourceUsageBanner({
    required String categoryLabel,
    required AppResourceUsage usage,
  }) {
    final palette = context.appPalette;
    Widget usageText(String label, String value) {
      return RichText(
        text: TextSpan(
          style: TextStyle(color: palette.textSecondary, fontSize: 12),
          children: [
            TextSpan(text: '$label: '),
            TextSpan(
              text: value,
              style: TextStyle(
                color: palette.textPrimary,
                fontWeight: FontWeight.w700,
              ),
            ),
          ],
        ),
      );
    }

    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
      decoration: BoxDecoration(
        color: palette.accentSoft,
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: palette.border),
      ),
      child: Wrap(
        spacing: 12,
        runSpacing: 6,
        children: [
          usageText(
            '$categoryLabel 用量',
            '${_formatBytes(usage.categorySize)} / ${usage.categoryCount} 个',
          ),
          usageText(
            '总用量',
            '${_formatBytes(usage.totalSize)} / ${usage.totalCount} 个',
          ),
        ],
      ),
    );
  }

  Future<bool> _confirmDeleteResourceItem(AppResourceItem item) async {
    final result = await showDialog<bool>(
      context: context,
      builder: (context) {
        return AlertDialog(
          title: const Text('删除资源'),
          content: Text('确定删除 ${item.fileName}？删除后资源库将不再保留该上传文件。'),
          actions: [
            TextButton(
              onPressed: () => Navigator.of(context).pop(false),
              child: const Text('取消'),
            ),
            FilledButton(
              onPressed: () => Navigator.of(context).pop(true),
              child: const Text('删除'),
            ),
          ],
        );
      },
    );
    return result == true;
  }

  Future<void> _deleteResourceItem(AppResourceItem item) async {
    if (item.fileId.isEmpty) {
      return;
    }
    try {
      await _runAuthed('Delete resource', (client) {
        return client.deleteResource(item.fileId);
      });
      _appendSystem('资源已删除：${item.fileName}');
      if (mounted) {
        setState(() => _status = 'Resource deleted');
      }
    } catch (err) {
      _appendSystem(_describeRequestError(err, operation: 'Delete resource'));
      rethrow;
    }
  }

  Future<void> _downloadResourceItem(AppResourceItem item) async {
    if (item.fileId.isEmpty) {
      return;
    }
    try {
      final filePath = await _attachmentPathForFileID(
        fileId: item.fileId,
        subdir: 'resource_downloads',
        prefix: item.category.isEmpty ? 'resource' : item.category,
        extension: item.fileFormat.isEmpty ? 'bin' : item.fileFormat,
      );
      await _runAuthed('Download attachment', (client) {
        return client.downloadAttachmentToFile(
          item.fileId,
          destinationPath: filePath,
          attachmentMeta: <String, dynamic>{
            'file_id': item.fileId,
            'file_name': item.fileName,
            'file_size': item.fileSize,
            'file_format': item.fileFormat,
          },
          onProgress: (received, total, resumed) {
            _updateDownloadStatus(
              label: item.fileName.isEmpty ? '资源' : item.fileName,
              receivedBytes: received,
              totalBytes: total,
              resumed: resumed,
            );
          },
        );
      });
      _clearDownloadStatus(successText: '资源已下载：$filePath');
    } catch (err) {
      _appendSystem(
        _describeRequestError(err, operation: 'Download attachment'),
      );
    }
  }

  String _resourceCategoryLabel(String category) {
    switch (category) {
      case 'live2d':
        return 'Live2D';
      case 'image':
        return '图片';
      case 'audio':
        return '音频';
      case 'video':
        return '视频';
      default:
        return '文件';
    }
  }

  IconData _resourceCategoryIcon(String category) {
    switch (category) {
      case 'live2d':
        return Icons.face_retouching_natural_outlined;
      case 'image':
        return Icons.image_outlined;
      case 'audio':
        return Icons.graphic_eq_rounded;
      case 'video':
        return Icons.movie_outlined;
      default:
        return Icons.insert_drive_file_outlined;
    }
  }

  Future<void> _pickAndSendImage(ImageSource source) async {
    if (_sessionToken.isEmpty && _refreshToken.isEmpty) {
      _appendSystem('Please login first.');
      return;
    }
    if (_sending || _recording) {
      return;
    }

    try {
      final picked = await _imagePicker.pickImage(
        source: source,
        imageQuality: 92,
      );
      if (picked == null) {
        return;
      }

      final bytes = await picked.readAsBytes();
      if (bytes.isEmpty) {
        _appendSystem('Selected image is empty.');
        return;
      }
      if (bytes.length > 4 * 1024 * 1024) {
        _appendSystem('Image too large. Please choose one under 4 MB.');
        return;
      }

      final fileName = picked.name.trim().isEmpty
          ? 'image_${DateTime.now().millisecondsSinceEpoch}.jpg'
          : picked.name.trim();
      final imageFormat = _detectImageFormat(fileName, bytes);
      final imageBase64 = base64Encode(bytes);
      final imagePath = picked.path.trim();
      final localMeta = <String, dynamic>{
        if (imagePath.isNotEmpty) 'image_path': imagePath,
        if (imagePath.isEmpty) 'image_base64': imageBase64,
        'image_format': imageFormat,
        'file_name': fileName,
        'input_mode': source == ImageSource.camera
            ? 'camera_image'
            : 'gallery_image',
        if (_currentGroupId.isNotEmpty) 'group_id': _currentGroupId,
        if (_currentGroupId.isNotEmpty) 'scope': 'group',
      };
      final sendMeta = <String, dynamic>{
        ...localMeta,
        'image_base64': imageBase64,
      };
      sendMeta.remove('image_path');

      _appendOutgoing('', messageType: 'image', meta: localMeta);
      _setSending(true);

      try {
        await _runAuthed('Send image', (client) {
          return client.sendAppMessage(
            '',
            messageType: 'image',
            meta: sendMeta,
          );
        });
        _setStatusText('Image sent');
      } finally {
        _setSending(false);
      }
    } catch (err) {
      _appendSystem(_describeRequestError(err, operation: 'Send image'));
    }
  }

  Future<String> _persistVoiceMessage({
    required List<int> bytes,
    required String extension,
  }) async {
    return _persistAttachmentBytes(
      bytes: bytes,
      subdir: 'voice_messages',
      prefix: 'voice',
      extension: extension,
    );
  }

  Future<String> _persistAttachmentBytes({
    required List<int> bytes,
    required String subdir,
    required String prefix,
    required String extension,
  }) async {
    final supportDir = await getApplicationSupportDirectory();
    final voiceDir = Directory(
      '${supportDir.path}${Platform.pathSeparator}$subdir',
    );
    if (!await voiceDir.exists()) {
      await voiceDir.create(recursive: true);
    }
    final file = File(
      '${voiceDir.path}${Platform.pathSeparator}${prefix}_${DateTime.now().millisecondsSinceEpoch}.$extension',
    );
    await file.writeAsBytes(bytes, flush: true);
    return file.path;
  }

  Future<String> _attachmentPathForFileID({
    required String fileId,
    required String subdir,
    required String prefix,
    required String extension,
  }) async {
    final supportDir = await getApplicationSupportDirectory();
    final targetDir = Directory(
      '${supportDir.path}${Platform.pathSeparator}$subdir',
    );
    if (!await targetDir.exists()) {
      await targetDir.create(recursive: true);
    }
    final safeFileID = fileId
        .replaceAll(RegExp(r'[^A-Za-z0-9._-]'), '_')
        .replaceAll('__', '_');
    final ext = extension.trim().isEmpty
        ? 'bin'
        : extension.trim().toLowerCase();
    return '${targetDir.path}${Platform.pathSeparator}${prefix}_$safeFileID.$ext';
  }

  void _setStatusText(String status) {
    if (!mounted || _status == status) {
      return;
    }
    setState(() {
      _status = status;
    });
  }

  void _setSending(bool sending) {
    if (!mounted || _sending == sending) {
      return;
    }
    setState(() {
      _sending = sending;
    });
  }

  void _updateDownloadStatus({
    required String label,
    required int receivedBytes,
    required int? totalBytes,
    required bool resumed,
  }) {
    final percent = totalBytes == null || totalBytes <= 0
        ? -1
        : ((receivedBytes * 100) / totalBytes).floor().clamp(0, 100);
    if (!mounted) {
      return;
    }
    if (_downloadStatusLabel == label && _downloadStatusPercent == percent) {
      return;
    }
    final progressText = totalBytes == null || totalBytes <= 0
        ? _formatBytes(receivedBytes)
        : '${_formatBytes(receivedBytes)} / ${_formatBytes(totalBytes)}';
    final resumeText = resumed ? '继续下载' : '下载中';
    setState(() {
      _downloadStatusLabel = label;
      _downloadStatusPercent = percent;
      _status = percent >= 0
          ? '$resumeText $label $percent% ($progressText)'
          : '$resumeText $label ($progressText)';
    });
  }

  void _clearDownloadStatus({String? successText}) {
    if (!mounted) {
      return;
    }
    final nextStatus = successText != null && successText.trim().isNotEmpty
        ? successText
        : _status;
    if (_downloadStatusLabel == null &&
        _downloadStatusPercent == -1 &&
        nextStatus == _status) {
      return;
    }
    setState(() {
      _downloadStatusLabel = null;
      _downloadStatusPercent = -1;
      _status = nextStatus;
    });
  }

  String _formatBytes(int bytes) {
    if (bytes < 1024) {
      return '$bytes B';
    }
    final kb = bytes / 1024;
    if (kb < 1024) {
      return '${kb.toStringAsFixed(kb >= 100 ? 0 : 1)} KB';
    }
    final mb = kb / 1024;
    if (mb < 1024) {
      return '${mb.toStringAsFixed(mb >= 100 ? 0 : 1)} MB';
    }
    final gb = mb / 1024;
    return '${gb.toStringAsFixed(gb >= 100 ? 0 : 1)} GB';
  }

  String _detectImageFormat(String fileName, List<int> bytes) {
    final lowerName = fileName.toLowerCase();
    if (lowerName.endsWith('.png')) {
      return 'png';
    }
    if (lowerName.endsWith('.webp')) {
      return 'webp';
    }
    if (lowerName.endsWith('.gif')) {
      return 'gif';
    }
    if (lowerName.endsWith('.bmp')) {
      return 'bmp';
    }
    if (bytes.length >= 4 &&
        bytes[0] == 0x89 &&
        bytes[1] == 0x50 &&
        bytes[2] == 0x4E &&
        bytes[3] == 0x47) {
      return 'png';
    }
    return 'jpg';
  }

  Future<void> _sendMessage() async {
    if (_sessionToken.isEmpty && _refreshToken.isEmpty) {
      _appendSystem('Please login first.');
      return;
    }
    final text = _messageController.text.trim();
    if (text.isEmpty) {
      return;
    }
    FocusScope.of(context).unfocus();
    _messageController.clear();
    final deviceContext = await _refreshCortanaDeviceContext(
      report: true,
      force: true,
    );
    final meta = <String, dynamic>{
      'device_context': ?deviceContext,
      if (_currentGroupId.isEmpty) ...<String, dynamic>{
        'conversation_mode': 'cortana',
        'input_mode': 'cortana_chat',
        'reply_mode': 'text',
      },
      if (_currentGroupId.isNotEmpty) ...<String, dynamic>{
        'group_id': _currentGroupId,
        'scope': 'group',
      },
    };
    _appendOutgoing(text, meta: meta.isEmpty ? null : meta);
    _setSending(true);
    try {
      await _runAuthed('Send message', (client) {
        return client.sendAppMessage(text, meta: meta);
      });
      _setStatusText('Message sent');
    } catch (err) {
      _appendSystem(_describeRequestError(err, operation: 'Send message'));
    } finally {
      _setSending(false);
    }
  }
}
