class PcmProcessor extends AudioWorkletProcessor {
  process(
    inputs: Float32Array[][],
    _outputs: Float32Array[][],
    _parameters: Record<string, Float32Array>,
  ): boolean {
    const input = inputs[0];
    // eslint-disable-next-line @typescript-eslint/no-unnecessary-condition
    if (!input || input.length === 0 || !input[0]) {
      return true;
    }

    const channelData = input[0];
    const frameCount = channelData.length;
    const pcmBuffer = new Int16Array(frameCount);

    for (let i = 0; i < frameCount; i++) {
      const sample = channelData[i];
      pcmBuffer[i] = sample < 0 ? sample * 32768 : sample * 32767;
    }

    const byteLength = pcmBuffer.byteLength;
    const arrayBuffer = new ArrayBuffer(byteLength);
    new Int16Array(arrayBuffer).set(pcmBuffer);

    this.port.postMessage({ buffer: arrayBuffer }, [arrayBuffer]);
    return true;
  }
}

registerProcessor("pcm-processor", PcmProcessor);
