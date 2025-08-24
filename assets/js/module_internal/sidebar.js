export const sidebarHandler = () => {
  const btns = document.querySelectorAll("[data-sidebar]");

  btns.forEach((btn)=>{
    const getId = btn.getAttribute("data-sidebar");
    const targetId = document.getElementById(getId);

    btn.addEventListener("click", ()=>{
      targetId.classList.toggle("sidebar--active");
    })
  })
}