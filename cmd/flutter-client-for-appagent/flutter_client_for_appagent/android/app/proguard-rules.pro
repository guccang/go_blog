# Vosk depends on JNA native code, which looks up Java members by exact names.
# If these classes or members are obfuscated, Model() initialization fails with
# "Can't obtain peer field ID for class com.sun.jna.Pointer".
-keep class com.sun.jna.** { *; }
-keep class org.vosk.** { *; }
-keepclassmembers class com.sun.jna.Pointer {
    long peer;
}
-keepclasseswithmembernames class * {
    native <methods>;
}

# Keep Flutter/plugin entry points stable when release minification is enabled.
-keep class io.flutter.** { *; }
-keep class io.flutter.plugins.** { *; }
-keep class com.pichillilorenzo.flutter_inappwebview_android.** { *; }

# Flutter's engine contains optional Play Store deferred-component hooks. This
# app does not declare dynamic feature modules, so the Play Core splitinstall
# types are not packaged and can be ignored by R8.
-dontwarn com.google.android.play.core.splitcompat.SplitCompatApplication
-dontwarn com.google.android.play.core.splitinstall.SplitInstallException
-dontwarn com.google.android.play.core.splitinstall.SplitInstallManager
-dontwarn com.google.android.play.core.splitinstall.SplitInstallManagerFactory
-dontwarn com.google.android.play.core.splitinstall.SplitInstallRequest$Builder
-dontwarn com.google.android.play.core.splitinstall.SplitInstallRequest
-dontwarn com.google.android.play.core.splitinstall.SplitInstallSessionState
-dontwarn com.google.android.play.core.splitinstall.SplitInstallStateUpdatedListener
-dontwarn com.google.android.play.core.tasks.OnFailureListener
-dontwarn com.google.android.play.core.tasks.OnSuccessListener
-dontwarn com.google.android.play.core.tasks.Task

# JNA includes desktop AWT helpers that are unreachable on Android. Vosk keeps
# the Android native path above, and these java.awt references are optional.
-dontwarn java.awt.Component
-dontwarn java.awt.GraphicsEnvironment
-dontwarn java.awt.HeadlessException
-dontwarn java.awt.Window
