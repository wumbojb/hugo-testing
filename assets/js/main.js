import { renderGiscus } from "./module_internal/render_giscus";
import { sidebarHandler } from "./module_internal/sidebar";
import { tabHandler } from "./module_internal/tab";
const main = () => {
  renderGiscus("light");
  sidebarHandler();
  tabHandler();
};

document.addEventListener("DOMContentLoaded", main);
