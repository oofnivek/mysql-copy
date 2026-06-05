// Modal
function openModal(id) {
    const modal = document.getElementById(id);
    modal.removeAttribute("hidden");
    modal.querySelector("input, button, select, textarea")?.focus();
}

function closeModal(id) {
    const modal = document.getElementById(id);
    modal.setAttribute("hidden", "");
    modal.querySelector("#connection-feedback").innerHTML = "";
}

document.addEventListener("keydown", (e) => {
    if (e.key === "Escape") {
        document.querySelectorAll(".modal:not([hidden])").forEach((m) => {
            closeModal(m.id);
        });
    }
});

// Close modal on successful connection
function onConnectionResponse(event) {
    if (event.detail.successful) {
        setTimeout(() => closeModal("modal-connection"), 1200);
    }
}

// HTMX
document.addEventListener("DOMContentLoaded", () => {
    document.body.addEventListener("htmx:beforeRequest", () => {
        document.body.classList.add("loading");
    });

    document.body.addEventListener("htmx:afterRequest", () => {
        document.body.classList.remove("loading");
    });
});
