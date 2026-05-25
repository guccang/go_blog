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
import 'package:sherpa_onnx/sherpa_onnx.dart' as sherpa;
import 'package:shared_preferences/shared_preferences.dart';
import 'package:speech_to_text/speech_to_text.dart' as stt;
import 'package:web_socket_channel/web_socket_channel.dart';

import 'core/platform/sherpa_keyword_tokenizer.dart';
import 'core/platform/sherpa_model_locator.dart';
import 'features/codegen/codegen_body.dart';
import 'features/codegen/models.dart';
import 'features/codegen/task_page.dart';
import 'features/cortana/domain/cortana_broadcast_queue.dart';
import 'features/cortana/history/cortana_history_page.dart';
import 'features/cortana/presentation/cortana_page.dart'
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
import 'shared/utils/speech_transcript_formatter.dart';
import 'version.g.dart';

part 'core/client/app_agent_client.dart';
part 'core/config/chat_constants.dart';
part 'core/debug/app_debug_copy.dart';
part 'core/debug/app_debug_recorder.dart';
part 'core/models/app_models.dart';
part 'core/platform/app_platform_services.dart';
part 'core/platform/sherpa_speech_engine.dart';
part 'features/chat/chat_codegen.dart';
part 'features/chat/chat_cortana.dart';
part 'features/chat/chat_live2d_config.dart';
part 'features/chat/chat_messages_history.dart';
part 'features/chat/chat_page_core.dart';
part 'features/chat/chat_ui_sections.dart';
part 'features/chat/chat_voice_attachments.dart';
part 'features/settings/app_storage_manager.dart';
part 'shared/theme/app_theme.dart';
part 'shared/widgets/message_widgets.dart';

void main() {
  runZonedGuarded(
    () {
      WidgetsFlutterBinding.ensureInitialized();
      FlutterError.onError = (FlutterErrorDetails details) {
        AppDebugRecorder.instance.recordFlutterError(details);
        addFlutterClientLog('FlutterError: ${details.exceptionAsString()}');
        if (details.stack != null) {
          addFlutterClientLog(details.stack.toString());
        }
        FlutterError.presentError(details);
      };
      PlatformDispatcher.instance.onError = (Object error, StackTrace stack) {
        AppDebugRecorder.instance.recordPlatformError(error, stack);
        addFlutterClientLog('PlatformError: $error\n$stack');
        return true;
      };
      runApp(const AppAgentClientApp());
    },
    (Object error, StackTrace stack) {
      AppDebugRecorder.instance.recordPlatformError(error, stack);
      addFlutterClientLog('ZoneError: $error\n$stack');
    },
  );
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
