import { z } from "zod";

export const SkillSchema = z.object({
  id: z.string().min(1),
  title: z.string().min(1),
  level: z.enum(["A1", "A2", "B1", "B2", "C1", "C2"]),
  locale: z.string().min(2),
  voice: z.string().min(1),
  persona: z.object({
    name: z.string().min(1),
    role: z.string().min(1),
  }),
  constraints: z.object({
    vocabulary: z.string().min(1),
    turn_length: z.string().min(1),
    corrections: z.string().min(1),
  }),
  opening_line: z.string().min(1),
  system_prompt: z.string().min(1),
});

export type Skill = z.infer<typeof SkillSchema>;

export const SkillListSchema = z.array(SkillSchema);
