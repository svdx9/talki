import { Hono } from "hono";
import { serve } from "@hono/node-server";
import { serveStatic } from "@hono/node-server/serve-static";
import { loadConfig } from "./config";
import { loadSkills } from "./skills/loader";

const skills = loadSkills();
console.log(`Loaded ${skills.length} skill(s): ${skills.map((s) => s.id).join(", ")}`);

const app = new Hono();

app.get("/api/health", (c) => {
  return c.json({ status: "ok", timestamp: new Date().toISOString() });
});

app.get("/api/skills", (c) => {
  return c.json(skills.map(({ id, title, level }) => ({ id, title, level })));
});

app.use("/*", serveStatic({ root: "./frontend/dist" }));

const config = loadConfig();
const port = config.PORT;

console.log(`Server starting on port ${port}...`);

serve({
  fetch: app.fetch,
  port,
});
