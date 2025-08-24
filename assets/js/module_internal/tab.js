export const tabHandler = () => {
  const btns = document.querySelectorAll("[data-tab-target]");
  const panes = document.querySelectorAll(".tab-pane");

  btns.forEach((btn) => {
    btn.addEventListener("click", () => {
      const target = btn.getAttribute("data-tab-target");
      panes.forEach((pane) => pane.classList.remove("tab-pane--active"));
      document.getElementById(target)?.classList.add("tab-pane--active");
    });
  });
};
