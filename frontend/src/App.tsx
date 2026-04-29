import { Component } from "solid-js";

const App: Component = () => {
  return (
    <div
      style={{
        "min-height": "100vh",
        display: "flex",
        "align-items": "center",
        "justify-content": "center",
        "font-family": "system-ui, sans-serif",
        background: "#f5f5f5",
      }}
    >
      <div style={{ "text-align": "center" }}>
        <h1 style={{ "font-size": "2.5rem", margin: "0 0 0.5rem" }}>Talki</h1>
        <p style={{ color: "#555", margin: 0 }}>Conversational French tutor</p>
      </div>
    </div>
  );
};

export default App;
