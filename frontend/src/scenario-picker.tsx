import type { Component } from "solid-js";
import { createSignal, onMount, For } from "solid-js";

interface SkillInfo {
  id: string;
  title: string;
  level: string;
}

interface ScenarioPickerProps {
  onSelect: (scenarioId: string) => void;
  disabled?: boolean;
}

export const ScenarioPicker: Component<ScenarioPickerProps> = (props) => {
  const [skills, setSkills] = createSignal<SkillInfo[]>([]);
  const [loading, setLoading] = createSignal(false);
  const [errorMsg, setErrorMsg] = createSignal("");

  const fetchSkills = async () => {
    setLoading(true);
    try {
      const res = await fetch("/api/skills");
      if (!res.ok) {
        throw new Error("Failed to fetch skills");
      }
      const data = (await res.json()) as SkillInfo[];
      setSkills(data);
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Unknown error";
      setErrorMsg(msg);
    } finally {
      setLoading(false);
    }
  };

  onMount(() => {
    fetchSkills().catch((err: unknown) => {
      console.error("Failed to fetch skills:", err);
    });
  });

  const handleChange = (e: Event) => {
    const target = e.currentTarget as HTMLSelectElement | null;
    const value = target?.value ?? "";
    if (value) {
      props.onSelect(value);
    }
  };

  return (
    <div style={{ display: "flex", "flex-direction": "column", "align-items": "center", gap: "0.5rem" }}>
      <label style={{ "font-weight": "500", color: "#333" }}>Choose a scenario:</label>
      <select
        onChange={handleChange}
        disabled={!!props.disabled || loading()}
        style={{
          padding: "0.5rem 1rem",
          "border-radius": "8px",
          "border": "1px solid #ccc",
          "font-size": "1rem",
          "min-width": "200px",
          background: "#fff",
          cursor: props.disabled || loading() ? "not-allowed" : "pointer",
          opacity: props.disabled || loading() ? 0.6 : 1,
        }}
      >
        <option value="" disabled selected>
          {loading() ? "Loading..." : "Select scenario..."}
        </option>
        <For each={skills()}>
          {(skill) => (
            <option value={skill.id}>
              {skill.title} ({skill.level})
            </option>
          )}
        </For>
      </select>
      {errorMsg() && <p style={{ color: "#ff3b30", margin: 0 }}>{errorMsg()}</p>}
    </div>
  );
};