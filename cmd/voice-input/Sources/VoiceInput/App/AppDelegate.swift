import AppKit
import Speech

class AppDelegate: NSObject, NSApplicationDelegate {
    private var statusBarController: StatusBarController?
    private var fnKeyMonitor: FnKeyMonitor?
    private var speechTranscriber: SpeechTranscriber?
    private var floatingWindowController: FloatingWindowController?
    private var textInjectionManager: TextInjectionManager?
    private var llmRefiner: LLMRefiner?
    private var settingsWindowController: SettingsWindowController?

    func applicationDidFinishLaunching(_ notification: Notification) {
        // Set app as UI element (no dock icon)
        NSApp.setActivationPolicy(.accessory)

        // Initialize components
        textInjectionManager = TextInjectionManager()
        llmRefiner = LLMRefiner()
        floatingWindowController = FloatingWindowController()
        speechTranscriber = SpeechTranscriber()
        speechTranscriber?.delegate = self
        statusBarController = StatusBarController()
        statusBarController?.delegate = self

        // Initialize Fn key monitor
        fnKeyMonitor = FnKeyMonitor()
        fnKeyMonitor?.delegate = self
        fnKeyMonitor?.startMonitoring()

        // Request speech recognition permission
        requestSpeechPermission()

        // Load saved language preference
        if let savedLanguage = UserDefaults.standard.string(forKey: "selectedLanguage") {
            speechTranscriber?.setLanguage(savedLanguage)
        }
    }

    func applicationWillTerminate(_ notification: Notification) {
        fnKeyMonitor?.stopMonitoring()
    }

    private func requestSpeechPermission() {
        SFSpeechRecognizer.requestAuthorization { status in
            DispatchQueue.main.async {
                switch status {
                case .authorized:
                    print("Speech recognition authorized")
                case .denied, .restricted, .notDetermined:
                    print("Speech recognition not authorized: \(status)")
                @unknown default:
                    break
                }
            }
        }

        // Request microphone permission
        AVCaptureDevice.requestAccess(for: .audio) { granted in
            print("Microphone access: \(granted)")
        }
    }
}

// MARK: - StatusBarControllerDelegate
extension AppDelegate: StatusBarControllerDelegate {
    func statusBarControllerDidRequestSettings() {
        showSettingsWindow()
    }

    func statusBarControllerDidToggleLLM(enabled: Bool) {
        llmRefiner?.isEnabled = enabled
        UserDefaults.standard.set(enabled, forKey: "llmEnabled")
    }

    func statusBarControllerDidChangeLanguage(_ language: String) {
        speechTranscriber?.setLanguage(language)
        UserDefaults.standard.set(language, forKey: "selectedLanguage")
    }

    private func showSettingsWindow() {
        if settingsWindowController == nil {
            settingsWindowController = SettingsWindowController()
            settingsWindowController?.llmRefiner = llmRefiner
        }
        settingsWindowController?.showWindow(nil)
        NSApp.activate(ignoringOtherApps: true)
    }
}

// MARK: - FnKeyMonitorDelegate
extension AppDelegate: FnKeyMonitorDelegate {
    func fnKeyPressed() {
        // Fn key pressed - start recording
        speechTranscriber?.startRecording()
        floatingWindowController?.showWindow()
        floatingWindowController?.updateStatus(.recording)
    }

    func fnKeyReleased() {
        // Fn key released - stop recording and process
        speechTranscriber?.stopRecording()
        floatingWindowController?.updateStatus(.processing)

        // Process the transcribed text
        processTranscribedText()
    }

    private func processTranscribedText() {
        guard let transcribedText = speechTranscriber?.transcribedText, !transcribedText.isEmpty else {
            floatingWindowController?.hideWindow()
            return
        }

        // Check if LLM refinement is enabled and configured
        if llmRefiner?.isEnabled == true && llmRefiner?.isConfigured == true {
            floatingWindowController?.updateStatus(.refining)

            llmRefiner?.refineText(transcribedText) { [weak self] refinedText in
                DispatchQueue.main.async {
                    let finalText = refinedText ?? transcribedText
                    self?.injectText(finalText)
                }
            }
        } else {
            injectText(transcribedText)
        }
    }

    private func injectText(_ text: String) {
        floatingWindowController?.updateTranscribedText(text)
        floatingWindowController?.updateStatus(.injecting)

        textInjectionManager?.injectText(text) { [weak self] success in
            DispatchQueue.main.async {
                self?.floatingWindowController?.hideWindow()
            }
        }
    }
}

// MARK: - SpeechTranscriberDelegate
extension AppDelegate: SpeechTranscriberDelegate {
    func speechTranscriber(_ transcriber: SpeechTranscriber, didUpdateTranscription text: String) {
        DispatchQueue.main.async {
            self.floatingWindowController?.updateTranscribedText(text)
        }
    }

    func speechTranscriber(_ transcriber: SpeechTranscriber, didUpdateAudioLevel level: Float) {
        DispatchQueue.main.async {
            self.floatingWindowController?.updateAudioLevel(level)
        }
    }
}

import AVFoundation
