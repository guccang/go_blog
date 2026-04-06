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
