import Foundation
import AppKit
import Carbon.HIToolbox

protocol FnKeyMonitorDelegate: AnyObject {
    func fnKeyPressed()
    func fnKeyReleased()
}

class FnKeyMonitor {
    weak var delegate: FnKeyMonitorDelegate?

    private var eventTap: CFMachPort?
    private var runLoopSource: CFRunLoopSource?
    private var isFnKeyDown = false

    // Fn key key code is 63 (VK_FN)
    private let fnKeyCode: CGKeyCode = 63

    func startMonitoring() {
        // Create event tap for key down and key up events
        let eventMask = (1 << CGEventType.keyDown.rawValue) | (1 << CGEventType.keyUp.rawValue)

        // Use a helper class to avoid C callback issues with Swift
        let callback: CGEventTapCallBack = { proxy, type, event, refcon in
            guard let refcon = refcon else { return Unmanaged.passRetained(event) }

            let monitor = Unmanaged<FnKeyMonitor>.fromOpaque(refcon).takeUnretainedValue()
            return monitor.handleEvent(proxy: proxy, type: type, event: event)
        }

        let refcon = UnsafeMutableRawPointer(Unmanaged.passUnretained(self).toOpaque())

        guard let tap = CGEvent.tapCreate(
            tap: .cgSessionEventTap,
            place: .headInsertEventTap,
            options: .defaultTap,
            eventsOfInterest: CGEventMask(eventMask),
            callback: callback,
            userInfo: refcon
        ) else {
            print("Failed to create event tap. Check Accessibility permissions.")
            // Try withcg eve
            return
        }

        eventTap = tap

        runLoopSource = CFMachPortCreateRunLoopSource(kCFAllocatorDefault, tap, 0)
        CFRunLoopAddSource(CFRunLoopGetCurrent(), runLoopSource, .commonModes)
        CGEvent.tapEnable(tap: tap, enable: true)
    }

    func stopMonitoring() {
        if let tap = eventTap {
            CGEvent.tapEnable(tap: tap, enable: false)
        }

        if let source = runLoopSource {
            CFRunLoopRemoveSource(CFRunLoopGetCurrent(), source, .commonModes)
        }

        runLoopSource = nil
        eventTap = nil
    }

    private func handleEvent(proxy: CGEventTapProxy, type: CGEventType, event: CGEvent) -> Unmanaged<CGEvent>? {
        // Handle tap disabled events
        if type == .tapDisabledByTimeout || type == .tapDisabledByUserInput {
            if let tap = eventTap {
                CGEvent.tapEnable(tap: tap, enable: true)
            }
            return Unmanaged.passRetained(event)
        }

        let keyCode = CGKeyCode(event.getIntegerValueField(.keyboardEventKeycode))

        // Check if this is an Fn key event
        if keyCode == fnKeyCode {
            let isKeyDown = (type == .keyDown)

            if isKeyDown && !isFnKeyDown {
                // Fn key just pressed
                isFnKeyDown = true
                DispatchQueue.main.async {
                    self.delegate?.fnKeyPressed()
                }
                // Suppress the Fn event to prevent emoji picker
                return nil
            } else if !isKeyDown && isFnKeyDown {
                // Fn key just released
                isFnKeyDown = false
                DispatchQueue.main.async {
                    self.delegate?.fnKeyReleased()
                }
                // Suppress the Fn event
                return nil
            }
        }

        return Unmanaged.passRetained(event)
    }
}
