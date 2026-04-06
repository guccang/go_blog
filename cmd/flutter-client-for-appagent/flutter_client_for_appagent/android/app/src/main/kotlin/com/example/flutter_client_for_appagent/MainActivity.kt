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
import org.vosk.Model
import org.vosk.Recognizer
import java.io.File
import java.io.FileInputStream
import java.io.FileOutputStream
import java.io.IOException
import java.io.RandomAccessFile
import java.util.concurrent.ExecutorService
import java.util.concurrent.Executors
import java.util.zip.ZipInputStream

class MainActivity : FlutterActivity() {
    private val channelName = "com.example.flutter_client_for_appagent/vosk"
    private val installerChannelName = "com.example.flutter_client_for_appagent/installer"
    private val zipChannelName = "com.example.flutter_client_for_appagent/zip"
    private val requiredModelFiles =
        listOf(
            "am/final.mdl",
            "conf/mfcc.conf",
            "conf/model.conf",
            "graph/HCLr.fst",
            "graph/Gr.fst",
        )
    private val optionalIvectorFiles =
        listOf("ivector/final.ie", "ivector/final.mat", "ivector/online_cmvn.conf")
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
        MethodChannel(flutterEngine.dartExecutor.binaryMessenger, zipChannelName)
            .setMethodCallHandler { call, result ->
                when (call.method) {
                    "extractZip" -> handleExtractZip(call, result)
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
                val modelDir = resolveModelDir(modelPath)
                if (modelDir == null) {
                    runOnUiThread {
                        result.success(
                            mapOf(
                                "ready" to false,
                                "message" to "Vosk model directory is incomplete: $modelPath",
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
                            "message" to "Vosk model loaded: ${modelDir.absolutePath}",
                        ),
                    )
                }
            } catch (err: Throwable) {
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

    private fun resolveModelDir(modelPath: String): File? {
        val modelDir = File(modelPath)
        if (!modelDir.exists() || !modelDir.isDirectory) {
            return null
        }
        if (isValidModelDir(modelDir)) {
            return modelDir
        }
        val childDirs = modelDir.listFiles()?.filter { it.isDirectory } ?: return null
        for (childDir in childDirs) {
            if (isValidModelDir(childDir)) {
                return childDir
            }
        }
        return null
    }

    private fun isValidModelDir(modelDir: File): Boolean {
        if (!requiredModelFiles.all { relativePath -> File(modelDir, relativePath).isFile }) {
            return false
        }
        val ivectorDir = File(modelDir, "ivector")
        if (ivectorDir.isDirectory) {
            return optionalIvectorFiles.all { relativePath -> File(modelDir, relativePath).isFile }
        }
        return true
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

    private fun handleExtractZip(call: MethodCall, result: MethodChannel.Result) {
        val zipPath = call.argument<String>("zipPath")?.trim().orEmpty()
        val destPath = call.argument<String>("destPath")?.trim().orEmpty()
        if (zipPath.isEmpty() || destPath.isEmpty()) {
            runOnUiThread {
                result.success(
                    mapOf(
                        "success" to false,
                        "error" to "Invalid arguments: zipPath or destPath is empty",
                    ),
                )
            }
            return
        }
        executor.execute {
            val tempDir = File("${destPath}.extracting")
            try {
                val zipFile = File(zipPath)
                val destDir = File(destPath)
                if (!zipFile.exists() || !zipFile.isFile) {
                    runOnUiThread {
                        result.success(
                            mapOf(
                                "success" to false,
                                "error" to "ZIP file not found: $zipPath",
                            ),
                        )
                    }
                    return@execute
                }
                prepareEmptyDirectory(tempDir)
                unzipToDirectory(zipFile, tempDir)
                if (resolveModelDir(tempDir.absolutePath) == null) {
                    throw IOException("Extracted Vosk model is incomplete")
                }
                moveDirectory(tempDir, destDir)
                val finalModelDir =
                    resolveModelDir(destDir.absolutePath)
                        ?: throw IOException("Moved Vosk model is incomplete")
                runOnUiThread {
                    result.success(
                        mapOf(
                            "success" to true,
                            "error" to "",
                            "modelPath" to finalModelDir.absolutePath,
                        ),
                    )
                }
            } catch (err: Throwable) {
                deleteIfExists(tempDir)
                runOnUiThread {
                    result.success(
                        mapOf(
                            "success" to false,
                            "error" to "Extract ZIP failed: ${err.message ?: err.javaClass.simpleName}",
                        ),
                    )
                }
            }
        }
    }

    private fun prepareEmptyDirectory(dir: File) {
        deleteIfExists(dir)
        if (!dir.mkdirs() && !dir.isDirectory) {
            throw IOException("Create directory failed: ${dir.absolutePath}")
        }
    }

    private fun deleteIfExists(dir: File) {
        if (dir.exists() && !dir.deleteRecursively()) {
            throw IOException("Delete directory failed: ${dir.absolutePath}")
        }
    }

    private fun moveDirectory(sourceDir: File, destDir: File) {
        deleteIfExists(destDir)
        if (!sourceDir.renameTo(destDir)) {
            throw IOException(
                "Move directory failed: ${sourceDir.absolutePath} -> ${destDir.absolutePath}",
            )
        }
    }

    private fun unzipToDirectory(zipFile: File, destDir: File) {
        val canonicalDestDir = destDir.canonicalFile
        val canonicalDestPrefix = "${canonicalDestDir.path}${File.separator}"
        ZipInputStream(FileInputStream(zipFile)).use { zis ->
            var entry = zis.nextEntry
            while (entry != null) {
                val outFile = File(destDir, entry.name).canonicalFile
                if (outFile.path != canonicalDestDir.path &&
                    !outFile.path.startsWith(canonicalDestPrefix)
                ) {
                    throw IOException("Unsafe ZIP entry: ${entry.name}")
                }
                if (entry.isDirectory) {
                    if (!outFile.mkdirs() && !outFile.isDirectory) {
                        throw IOException("Create directory failed: ${outFile.absolutePath}")
                    }
                } else {
                    outFile.parentFile?.let { parent ->
                        if (!parent.mkdirs() && !parent.isDirectory) {
                            throw IOException("Create directory failed: ${parent.absolutePath}")
                        }
                    }
                    FileOutputStream(outFile).use { fos ->
                        val buffer = ByteArray(8192)
                        var len: Int
                        while (zis.read(buffer).also { len = it } > 0) {
                            fos.write(buffer, 0, len)
                        }
                        fos.fd.sync()
                    }
                }
                zis.closeEntry()
                entry = zis.nextEntry
            }
        }
    }
}
