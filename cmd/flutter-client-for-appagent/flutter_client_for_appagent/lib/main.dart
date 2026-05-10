import 'dart:async';
import 'dart:collection';
import 'dart:convert'
    show JsonEncoder, base64Decode, base64Encode, jsonDecode, jsonEncode, utf8;
import 'dart:io';
import 'dart:math' as math;
import 'dart:ui'
    show ImageByteFormat, PlatformDispatcher, instantiateImageCodec;

import 'package:archive/archive.dart';
import 'package:audioplayers/audioplayers.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_inappwebview/flutter_inappwebview.dart'
    hide AndroidOptions;
import 'package:http/http.dart' as http;
import 'package:image_picker/image_picker.dart';
import 'package:path_provider/path_provider.dart';
import 'package:record/record.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:speech_to_text/speech_to_text.dart' as stt;
import 'package:web_socket_channel/web_socket_channel.dart';

import 'codegen/codegen_body.dart';
import 'codegen/models.dart';
import 'cortana_broadcast_queue.dart';
import 'cortana_history_page.dart';
import 'cortana_page.dart'
    show
        CortanaDisplayMode,
        CortanaModelViewTransform,
        CortanaPage,
        CortanaPageState,
        CortanaReplayItem,
        CortanaReplyPayload,
        CortanaSettings,
        CortanaSuggestedReply,
        FlutterClientLogEntry,
        addFlutterClientLog,
        flutterClientLogs;
import 'speech_transcript_formatter.dart';
import 'version.g.dart';
import 'vosk_model_locator.dart';

part 'app_theme.dart';
part 'app_models.dart';
part 'app_platform_services.dart';
part 'app_storage_manager.dart';
part 'app_agent_client.dart';
part 'chat_constants.dart';
part 'chat_page_core.dart';
part 'chat_live2d_config.dart';
part 'chat_codegen.dart';
part 'chat_cortana.dart';
part 'chat_messages_history.dart';
part 'chat_voice_attachments.dart';
part 'chat_ui_sections.dart';
part 'message_widgets.dart';

void main() {
  FlutterError.onError = (FlutterErrorDetails details) {
    addFlutterClientLog('FlutterError: ${details.exceptionAsString()}');
    if (details.stack != null) {
      addFlutterClientLog(details.stack.toString());
    }
    FlutterError.presentError(details);
  };
  PlatformDispatcher.instance.onError = (Object error, StackTrace stack) {
    addFlutterClientLog('PlatformError: $error\n$stack');
    return false;
  };
  runApp(const AppAgentClientApp());
}

bool get _isAndroidHost => !kIsWeb && Platform.isAndroid;
bool get _isWindowsHost => !kIsWeb && Platform.isWindows;

String _platformLabel() {
  if (kIsWeb) return 'web';
  if (Platform.isAndroid) return 'android';
  if (Platform.isIOS) return 'ios';
  if (Platform.isWindows) return 'windows';
  if (Platform.isMacOS) return 'macos';
  if (Platform.isLinux) return 'linux';
  return 'unknown';
}
