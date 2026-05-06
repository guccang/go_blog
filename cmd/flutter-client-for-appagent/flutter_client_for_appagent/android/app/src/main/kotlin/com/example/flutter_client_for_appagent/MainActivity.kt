package com.example.flutter_client_for_appagent

import android.Manifest
import android.app.Activity
import android.content.ActivityNotFoundException
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.database.Cursor
import android.location.Location
import android.location.LocationListener
import android.location.LocationManager
import android.net.Uri
import android.os.Bundle
import android.os.Build
import android.os.Handler
import android.os.Looper
import android.provider.OpenableColumns
import android.provider.Settings
import androidx.core.app.ActivityCompat
import androidx.core.content.FileProvider
import androidx.core.content.ContextCompat
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
    private val locationChannelName = "com.example.flutter_client_for_appagent/location"
    private val filePickerChannelName = "com.example.flutter_client_for_appagent/file_picker"
    private val locationPermissionRequestCode = 40701
    private val filePickerRequestCode = 40702
    private var pendingLocationResult: MethodChannel.Result? = null
    private var pendingFilePickerResult: MethodChannel.Result? = null
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
        MethodChannel(flutterEngine.dartExecutor.binaryMessenger, locationChannelName)
            .setMethodCallHandler { call, result ->
                when (call.method) {
                    "getCurrentLocation" -> handleGetCurrentLocation(result)
                    else -> result.notImplemented()
                }
            }
        MethodChannel(flutterEngine.dartExecutor.binaryMessenger, filePickerChannelName)
            .setMethodCallHandler { call, result ->
                when (call.method) {
                    "pickFile" -> handlePickFile(result)
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

    override fun onRequestPermissionsResult(
        requestCode: Int,
        permissions: Array<out String>,
        grantResults: IntArray,
    ) {
        super.onRequestPermissionsResult(requestCode, permissions, grantResults)
        if (requestCode != locationPermissionRequestCode) {
            return
        }
        val result = pendingLocationResult ?: return
        pendingLocationResult = null
        if (grantResults.any { it == PackageManager.PERMISSION_GRANTED }) {
            resolveCurrentLocation(result)
        } else {
            result.success(
                mapOf(
                    "available" to false,
                    "permission" to "denied",
                    "message" to "Location permission denied",
                    "timestamp" to System.currentTimeMillis(),
                ),
            )
        }
    }

    override fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?) {
        super.onActivityResult(requestCode, resultCode, data)
        if (requestCode != filePickerRequestCode) {
            return
        }
        val result = pendingFilePickerResult ?: return
        pendingFilePickerResult = null
        if (resultCode != Activity.RESULT_OK) {
            result.success(null)
            return
        }
        val uri = data?.data
        if (uri == null) {
            result.success(null)
            return
        }
        executor.execute {
            try {
                val copiedFile = copyPickedFileToCache(uri)
                runOnUiThread {
                    result.success(
                        mapOf(
                            "path" to copiedFile.absolutePath,
                            "name" to copiedFile.name,
                        ),
                    )
                }
            } catch (err: Throwable) {
                runOnUiThread {
                    result.error(
                        "copy_failed",
                        err.message ?: err.javaClass.simpleName,
                        null,
                    )
                }
            }
        }
    }

    private fun handlePickFile(result: MethodChannel.Result) {
        if (pendingFilePickerResult != null) {
            result.error("file_picker_busy", "File picker is already running", null)
            return
        }
        val intent =
            Intent(Intent.ACTION_OPEN_DOCUMENT).apply {
                addCategory(Intent.CATEGORY_OPENABLE)
                type = "*/*"
                addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
            }
        pendingFilePickerResult = result
        try {
            startActivityForResult(intent, filePickerRequestCode)
        } catch (_: ActivityNotFoundException) {
            pendingFilePickerResult = null
            result.error("file_picker_unavailable", "No file manager available", null)
        }
    }

    private fun copyPickedFileToCache(uri: Uri): File {
        val originalName = resolveDisplayName(uri).ifEmpty {
            "picked_${System.currentTimeMillis()}.bin"
        }
        val safeName = originalName.replace(Regex("""[\\/:*?"<>|]"""), "_")
        val targetDir = File(cacheDir, "picked_uploads/${System.currentTimeMillis()}").apply {
            mkdirs()
        }
        val targetFile = File(targetDir, safeName)
        val input =
            contentResolver.openInputStream(uri)
                ?: throw IOException("Cannot open selected file")
        input.use { source ->
            FileOutputStream(targetFile).use { output ->
                source.copyTo(output)
            }
        }
        return targetFile
    }

    private fun resolveDisplayName(uri: Uri): String {
        var cursor: Cursor? = null
        try {
            cursor =
                contentResolver.query(
                    uri,
                    arrayOf(OpenableColumns.DISPLAY_NAME),
                    null,
                    null,
                    null,
                )
            if (cursor != null && cursor.moveToFirst()) {
                val index = cursor.getColumnIndex(OpenableColumns.DISPLAY_NAME)
                if (index >= 0) {
                    return cursor.getString(index)?.trim().orEmpty()
                }
            }
        } catch (_: Throwable) {
        } finally {
            cursor?.close()
        }
        return uri.lastPathSegment?.trim().orEmpty()
    }

    private fun handleGetCurrentLocation(result: MethodChannel.Result) {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M && !hasLocationPermission()) {
            if (pendingLocationResult != null) {
                result.error("location_busy", "Location permission request is already running", null)
                return
            }
            pendingLocationResult = result
            ActivityCompat.requestPermissions(
                this,
                arrayOf(
                    Manifest.permission.ACCESS_FINE_LOCATION,
                    Manifest.permission.ACCESS_COARSE_LOCATION,
                ),
                locationPermissionRequestCode,
            )
            return
        }
        resolveCurrentLocation(result)
    }

    private fun hasLocationPermission(): Boolean {
        return ContextCompat.checkSelfPermission(
            this,
            Manifest.permission.ACCESS_FINE_LOCATION,
        ) == PackageManager.PERMISSION_GRANTED ||
            ContextCompat.checkSelfPermission(
                this,
                Manifest.permission.ACCESS_COARSE_LOCATION,
            ) == PackageManager.PERMISSION_GRANTED
    }

    private fun resolveCurrentLocation(result: MethodChannel.Result) {
        try {
            if (!hasLocationPermission()) {
                result.success(
                    mapOf(
                        "available" to false,
                        "permission" to "denied",
                        "timestamp" to System.currentTimeMillis(),
                    ),
                )
                return
            }
            val locationManager = getSystemService(Context.LOCATION_SERVICE) as LocationManager
            val enabledProviders = locationManager.getProviders(true)
            if (enabledProviders.isEmpty()) {
                result.success(
                    mapOf(
                        "available" to false,
                        "permission" to "granted",
                        "provider_enabled" to false,
                        "message" to "No location provider enabled",
                        "timestamp" to System.currentTimeMillis(),
                    ),
                )
                return
            }

            var bestLocation: Location? = null
            for (provider in enabledProviders) {
                val location = try {
                    locationManager.getLastKnownLocation(provider)
                } catch (_: SecurityException) {
                    null
                }
                val currentBest = bestLocation
                if (location != null && (currentBest == null || location.time > currentBest.time)) {
                    bestLocation = location
                }
            }

            val location = bestLocation
            if (location == null || System.currentTimeMillis() - location.time > 60_000L) {
                requestFreshLocation(locationManager, enabledProviders, bestLocation, result)
                return
            }

            result.success(locationPayload(location))
        } catch (err: Throwable) {
            result.success(
                mapOf(
                    "available" to false,
                    "permission" to "unknown",
                    "message" to (err.message ?: err.javaClass.simpleName),
                    "timestamp" to System.currentTimeMillis(),
                ),
            )
        }
    }

    private fun requestFreshLocation(
        locationManager: LocationManager,
        enabledProviders: List<String>,
        fallbackLocation: Location?,
        result: MethodChannel.Result,
    ) {
        val provider = when {
            enabledProviders.contains(LocationManager.GPS_PROVIDER) -> LocationManager.GPS_PROVIDER
            enabledProviders.contains(LocationManager.NETWORK_PROVIDER) -> LocationManager.NETWORK_PROVIDER
            else -> enabledProviders.first()
        }
        val handler = Handler(Looper.getMainLooper())
        var completed = false
        lateinit var listener: LocationListener
        fun finish(payload: Map<String, Any?>) {
            if (completed) {
                return
            }
            completed = true
            try {
                locationManager.removeUpdates(listener)
            } catch (_: Throwable) {
            }
            result.success(payload)
        }
        listener = object : LocationListener {
            override fun onLocationChanged(location: Location) {
                finish(locationPayload(location))
            }
        }
        handler.postDelayed({
            val fallback = fallbackLocation
            if (fallback != null) {
                finish(
                    locationPayload(
                        fallback,
                        "fresh_location_timeout_using_last_known",
                    ),
                )
            } else {
                finish(
                    mapOf(
                        "available" to false,
                        "permission" to "granted",
                        "provider_enabled" to true,
                        "message" to "No recent location fix",
                        "timestamp" to System.currentTimeMillis(),
                    ),
                )
            }
        }, 8_000L)
        try {
            locationManager.requestSingleUpdate(provider, listener, Looper.getMainLooper())
        } catch (_: SecurityException) {
            finish(
                mapOf(
                    "available" to false,
                    "permission" to "denied",
                    "timestamp" to System.currentTimeMillis(),
                ),
            )
        }
    }

    private fun locationPayload(location: Location, message: String? = null): Map<String, Any?> {
        return mapOf(
            "available" to true,
            "permission" to "granted",
            "provider_enabled" to true,
            "latitude" to location.latitude,
            "longitude" to location.longitude,
            "accuracy_m" to location.accuracy.toDouble(),
            "provider" to location.provider.orEmpty(),
            "location_time" to location.time,
            "timestamp" to System.currentTimeMillis(),
            "message" to message,
        ).filterValues { it != null }
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
