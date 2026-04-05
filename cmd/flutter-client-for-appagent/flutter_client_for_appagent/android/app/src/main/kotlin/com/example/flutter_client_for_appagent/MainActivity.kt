package com.example.flutter_client_for_appagent

import android.content.Intent
import android.net.Uri
import android.os.Bundle
import android.os.Build
import android.provider.Settings
import androidx.core.content.FileProvider
import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.MethodCall
import io.flutter.plugin.common.MethodChannel
import org.json.JSONObject
import org.vosk.LibVosk
import org.vosk.Model
import org.vosk.Recognizer
import java.io.File
import java.io.RandomAccessFile
import java.util.concurrent.ExecutorService
import java.util.concurrent.Executors

class MainActivity : FlutterActivity() {
    private val channelName = "com.example.flutter_client_for_appagent/vosk"
    private val installerChannelName = "com.example.flutter_client_for_appagent/installer"
    private val executor: ExecutorService = Executors.newSingleThreadExecutor()
    @Volatile private var voskModel: Model? = null

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
    }

    override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)
        MethodChannel(flutterEngine.dartExecutor.binaryMessenger, channelName)
            .setMethodCallHandler { call, result ->
                when (call.method) {
                    "initialize" -> handleInitialize(call, result)
                    "transcribeFile" -> handleTranscribeFile(call, result)
                    else -> result.notImplemented()
                }
            }
        MethodChannel(flutterEngine.dartExecutor.binaryMessenger, installerChannelName)
            .setMethodCallHandler { call, result ->
                when (call.method) {
                    "installApk" -> handleInstallApk(call, result)
                    else -> result.notImplemented()
                }
            }
    }

    override fun onDestroy() {
        super.onDestroy()
        executor.shutdownNow()
        voskModel?.close()
        voskModel = null
    }

    private fun handleInitialize(call: MethodCall, result: MethodChannel.Result) {
        val modelPath = call.argument<String>("modelPath")?.trim().orEmpty()
        if (modelPath.isEmpty()) {
            result.success(
                mapOf(
                    "ready" to false,
                    "message" to "Vosk model path is empty",
                ),
            )
            return
        }
        executor.execute {
            try {
                val modelDir = File(modelPath)
                if (!modelDir.exists() || !modelDir.isDirectory) {
                    runOnUiThread {
                        result.success(
                            mapOf(
                                "ready" to false,
                                "message" to "Vosk model directory not found: $modelPath",
                            ),
                        )
                    }
                    return@execute
                }
                val newModel = Model(modelDir.absolutePath)
                val oldModel = voskModel
                voskModel = newModel
                oldModel?.close()
                runOnUiThread {
                    result.success(
                        mapOf(
                            "ready" to true,
                            "message" to "Vosk model loaded",
                        ),
                    )
                }
            } catch (err: Exception) {
                runOnUiThread {
                    result.success(
                        mapOf(
                            "ready" to false,
                            "message" to "Load Vosk model failed: ${err.message ?: err.javaClass.simpleName}",
                        ),
                    )
                }
            }
        }
    }

    private fun handleTranscribeFile(call: MethodCall, result: MethodChannel.Result) {
        val audioPath = call.argument<String>("audioPath")?.trim().orEmpty()
        val model = voskModel
        if (model == null) {
            result.error("vosk_not_ready", "Vosk model is not initialized", null)
            return
        }
        if (audioPath.isEmpty()) {
            result.error("invalid_audio", "Audio path is empty", null)
            return
        }
        executor.execute {
            try {
                val wavFile = File(audioPath)
                if (!wavFile.exists() || !wavFile.isFile) {
                    runOnUiThread {
                        result.error("invalid_audio", "Audio file not found: $audioPath", null)
                    }
                    return@execute
                }
                val sampleRate = readWavSampleRate(wavFile)
                val text = transcribeWavFile(model, wavFile, sampleRate)
                runOnUiThread {
                    result.success(
                        mapOf(
                            "text" to text,
                        ),
                    )
                }
            } catch (err: Exception) {
                runOnUiThread {
                    result.error(
                        "transcribe_failed",
                        err.message ?: err.javaClass.simpleName,
                        null,
                    )
                }
            }
        }
    }

    private fun handleInstallApk(call: MethodCall, result: MethodChannel.Result) {
        val apkPath = call.argument<String>("apkPath")?.trim().orEmpty()
        if (apkPath.isEmpty()) {
            result.error("invalid_apk", "APK path is empty", null)
            return
        }
        val apkFile = File(apkPath)
        if (!apkFile.exists() || !apkFile.isFile) {
            result.error("invalid_apk", "APK file not found: $apkPath", null)
            return
        }

        try {
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O && !packageManager.canRequestPackageInstalls()) {
                val settingsIntent = Intent(
                    Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES,
                    Uri.parse("package:$packageName"),
                ).apply {
                    addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                }
                startActivity(settingsIntent)
                result.success(
                    mapOf(
                        "started" to false,
                        "status" to "permission_required",
                    ),
                )
                return
            }

            val apkUri = FileProvider.getUriForFile(
                this,
                "${applicationContext.packageName}.fileprovider",
                apkFile,
            )
            val installIntent = Intent(Intent.ACTION_VIEW).apply {
                setDataAndType(apkUri, "application/vnd.android.package-archive")
                addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
            }
            startActivity(installIntent)
            result.success(
                mapOf(
                    "started" to true,
                    "status" to "install_intent_sent",
                ),
            )
        } catch (err: Exception) {
            result.error(
                "install_failed",
                err.message ?: err.javaClass.simpleName,
                null,
            )
        }
    }

    private fun transcribeWavFile(model: Model, wavFile: File, sampleRate: Float): String {
        RandomAccessFile(wavFile, "r").use { raf ->
            if (raf.length() <= 44) {
                return ""
            }
            raf.seek(44)
            Recognizer(model, sampleRate).use { recognizer ->
                val buffer = ByteArray(4096)
                while (true) {
                    val read = raf.read(buffer)
                    if (read <= 0) {
                        break
                    }
                    recognizer.acceptWaveForm(buffer, read)
                }
                val finalJson = JSONObject(recognizer.finalResult)
                return finalJson.optString("text", "").trim()
            }
        }
    }

    private fun readWavSampleRate(wavFile: File): Float {
        RandomAccessFile(wavFile, "r").use { raf ->
            if (raf.length() < 28) {
                return 16000f
            }
            raf.seek(24)
            val bytes = ByteArray(4)
            raf.readFully(bytes)
            val value =
                (bytes[0].toInt() and 0xFF) or
                    ((bytes[1].toInt() and 0xFF) shl 8) or
                    ((bytes[2].toInt() and 0xFF) shl 16) or
                    ((bytes[3].toInt() and 0xFF) shl 24)
            return if (value > 0) value.toFloat() else 16000f
        }
    }
}
