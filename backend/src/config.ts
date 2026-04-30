import { z } from "zod";
import { config as dotenvConfig } from "dotenv";
import { resolve, dirname } from "path";
import { fileURLToPath } from "url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

dotenvConfig({ path: resolve(__dirname, "../../.env") });

const configSchema = z.object({
  DEEPGRAM_API_KEY: z.string().min(1, "Deepgram API key required"),
  ANTHROPIC_API_KEY: z.string().min(1, "Anthropic API key required"),
  ELEVENLABS_API_KEY: z.string().min(1, "ElevenLabs API key required"),
  PORT: z.coerce.number().default(8787),
});

export type Config = z.infer<typeof configSchema>;

export function loadConfig(): Config {
  const env = {
    DEEPGRAM_API_KEY: process.env.DEEPGRAM_API_KEY,
    ANTHROPIC_API_KEY: process.env.ANTHROPIC_API_KEY,
    ELEVENLABS_API_KEY: process.env.ELEVENLABS_API_KEY,
    PORT: process.env.PORT,
  };

  return configSchema.parse(env);
}
