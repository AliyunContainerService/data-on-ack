import mermaid from "https://cdn.jsdelivr.net/npm/mermaid@11.15.0/dist/mermaid.esm.min.mjs";

const diagrams = Array.from(
  document.querySelectorAll(
    ".prose code.language-mermaid, .prose .language-mermaid code"
  )
).map((code, index) => {
  const source = code.textContent.trim();
  const container = document.createElement("pre");
  const wrapper = code.closest("div.language-mermaid") || code.closest("pre");

  container.className = "mermaid mermaid-loading";
  container.id = `mermaid-diagram-${index + 1}`;
  container.textContent = source;
  container.setAttribute("aria-label", `Mermaid diagram ${index + 1}`);
  wrapper.replaceWith(container);

  return { container, source };
});

if (diagrams.length > 0) {
  mermaid.initialize({
    startOnLoad: false,
    securityLevel: "strict",
    theme: "base",
    fontFamily: "Inter, ui-sans-serif, system-ui, sans-serif",
    themeVariables: {
      primaryColor: "#fff7ed",
      primaryBorderColor: "#ff6a00",
      primaryTextColor: "#172033",
      lineColor: "#64748b",
      secondaryColor: "#eff6ff",
      tertiaryColor: "#f8fafc",
    },
  });

  for (const { container, source } of diagrams) {
    try {
      await mermaid.run({ nodes: [container] });
      container.classList.remove("mermaid-loading");
    } catch (error) {
      container.classList.remove("mermaid-loading");
      container.classList.add("mermaid-error");
      container.textContent = source;
      console.error("Failed to render Mermaid diagram", error);
    }
  }
}
