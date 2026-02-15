// RelayStation CoreML Sound Classifier
//
// Reads 16kHz mono float32 PCM audio from stdin and classifies sounds
// using Apple's built-in SoundAnalysis framework. Runs on the Neural Engine
// (M1/M2/M3) or GPU automatically via CoreML.
//
// Build:
//   swiftc -O -framework SoundAnalysis -framework AVFoundation \
//     -framework CoreMedia main.swift -o soundclassifier
//
// Usage with FFmpeg:
//   ffmpeg -i <stream> -vn -f f32le -acodec pcm_f32le -ac 1 -ar 16000 pipe:1 \
//     | ./soundclassifier
//
// Output: one JSON line per second to stdout:
//   {"label":"speech","confidence":0.92,"time":1.00}

import Foundation
import SoundAnalysis
import AVFoundation
import CoreMedia

// MARK: - Results observer

class ClassifierObserver: NSObject, SNResultsObserving {

    // We're interested in speech, music, and silence-like categories
    private let interestingLabels: Set<String> = [
        "speech", "music", "silence", "crowd", "cheering",
        "applause", "vehicle", "engine", "laughter",
        "television", "radio", "jingle"
    ]

    func request(_ request: SNRequest, didProduce result: SNResult) {
        guard let result = result as? SNClassificationResult else { return }

        // Get top classification
        guard let top = result.classifications.first else { return }

        // Find top "interesting" classification
        var bestLabel = top.identifier
        var bestConf = top.confidence

        for c in result.classifications {
            if interestingLabels.contains(c.identifier) && c.confidence > 0.1 {
                bestLabel = c.identifier
                bestConf = c.confidence
                break
            }
        }

        let json: [String: Any] = [
            "label": bestLabel,
            "confidence": round(bestConf * 1000) / 1000,
            "time": round(result.timeRange.start.seconds * 100) / 100,
            "top3": Array(result.classifications.prefix(3).map { [
                "label": $0.identifier,
                "conf": round($0.confidence * 1000) / 1000
            ] as [String: Any] })
        ]

        if let data = try? JSONSerialization.data(withJSONObject: json),
           let str = String(data: data, encoding: .utf8) {
            print(str)
            fflush(stdout)
        }
    }

    func request(_ request: SNRequest, didFailWithError error: Error) {
        fputs("[CoreML] Error: \(error.localizedDescription)\n", stderr)
    }

    func requestDidComplete(_ request: SNRequest) {
        fputs("[CoreML] Classification complete\n", stderr)
    }
}

// MARK: - Main

fputs("[CoreML] Sound classifier starting (Neural Engine / GPU accelerated)\n", stderr)

let sampleRate: Double = 16000
guard let format = AVAudioFormat(
    commonFormat: .pcmFormatFloat32,
    sampleRate: sampleRate,
    channels: 1,
    interleaved: false
) else {
    fputs("[CoreML] Failed to create audio format\n", stderr)
    exit(1)
}

let analyzer: SNAudioStreamAnalyzer
do {
    analyzer = try SNAudioStreamAnalyzer(format: format)
} catch {
    fputs("[CoreML] Failed to create analyzer: \(error)\n", stderr)
    exit(1)
}

let observer = ClassifierObserver()

do {
    let request = try SNClassifySoundRequest(classifierIdentifier: .version1)
    request.windowDuration = CMTimeMakeWithSeconds(1.5, preferredTimescale: 48000)
    request.overlapFactor = 0.5
    try analyzer.add(request, withObserver: observer)
    fputs("[CoreML] Classifier ready (window: 1.5s, overlap: 50%)\n", stderr)
} catch {
    fputs("[CoreML] Failed to create classification request: \(error)\n", stderr)
    fputs("[CoreML] This requires macOS 12+ with SoundAnalysis framework\n", stderr)
    exit(1)
}

// Read PCM audio from stdin in 1-second chunks
let samplesPerChunk = Int(sampleRate) // 1 second
let bytesPerSample = MemoryLayout<Float>.size
let bytesPerChunk = samplesPerChunk * bytesPerSample
var framePosition: AVAudioFramePosition = 0
var chunksProcessed = 0

fputs("[CoreML] Reading 16kHz mono float32 PCM from stdin...\n", stderr)

while true {
    let data = FileHandle.standardInput.readData(ofLength: bytesPerChunk)
    if data.isEmpty { break }

    let sampleCount = data.count / bytesPerSample
    guard let pcmBuffer = AVAudioPCMBuffer(
        pcmFormat: format,
        frameCapacity: AVAudioFrameCount(sampleCount)
    ) else { continue }

    pcmBuffer.frameLength = AVAudioFrameCount(sampleCount)

    data.withUnsafeBytes { raw in
        guard let src = raw.baseAddress?.assumingMemoryBound(to: Float.self) else { return }
        pcmBuffer.floatChannelData![0].update(from: src, count: sampleCount)
    }

    analyzer.analyze(pcmBuffer, atAudioFramePosition: framePosition)
    framePosition += AVAudioFramePosition(sampleCount)
    chunksProcessed += 1

    if chunksProcessed % 60 == 0 {
        fputs("[CoreML] Processed \(chunksProcessed)s of audio\n", stderr)
    }
}

fputs("[CoreML] Stream ended after \(chunksProcessed)s\n", stderr)
analyzer.completeAnalysis()

// Give time for final callbacks
RunLoop.current.run(until: Date(timeIntervalSinceNow: 1.0))
