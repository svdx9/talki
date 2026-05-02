import { z } from "zod";
import { config as dotenvConfig } from "dotenv";
import { resolve, dirname } from "path";
import { fileURLToPath } from "url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

dotenvConfig({ path: resolve(__dirname, "../../.env") });

const configSchema = z.object({
  MISTRAL_API_KEY: z.string().min(1, "Mistral API key required"),
  ANTHROPIC_API_KEY: z.string().min(1, "Anthropic API key required"),
  MISTRAL_VOICE_ID: z.string().default("female-1"),
  PORT: z.coerce.number().default(8787),
  DEBUG_WS: z.coerce.boolean().default(false),
});

export type Config = z.infer<typeof configSchema>;

export function loadConfig(): Config {
  const env = {
    MISTRAL_API_KEY: process.env.MISTRAL_API_KEY,
    ANTHROPIC_API_KEY: process.env.ANTHROPIC_API_KEY,
    MISTRAL_VOICE_ID: process.env.MISTRAL_VOICE_ID,
    PORT: process.env.PORT,
    DEBUG_WS: process.env.DEBUG_WS,
  };

  return configSchema.parse(env);
}