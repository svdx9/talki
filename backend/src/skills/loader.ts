import { readdirSync, readFileSync } from "fs";
import { join, resolve, dirname } from "path";
import { fileURLToPath } from "url";
import yaml from "js-yaml";
import { SkillSchema, Skill } from "./schema.js";

export type { Skill };

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

const CATALOG_DIR = resolve(__dirname, "catalog");

export function loadSkills(): Skill[] {
  const files = readdirSync(CATALOG_DIR).filter((f) => f.endsWith(".yaml") || f.endsWith(".yml"));

  if (files.length === 0) {
    throw new Error(`No YAML files found in ${CATALOG_DIR}`);
  }

  const skills: Skill[] = [];

  for (const file of files) {
    const filePath = join(CATALOG_DIR, file);
    const content = readFileSync(filePath, "utf-8");

    let parsed: unknown;
    try {
      parsed = yaml.load(content);
    } catch (e) {
      throw new Error(`Failed to parse YAML file ${file}: ${e}`);
    }

    const result = SkillSchema.safeParse(parsed);
    if (!result.success) {
      throw new Error(`Invalid skill file ${file}: ${result.error.message}`);
    }

    skills.push(result.data);
  }

  return skills;
}
