// Dropdown
function toggleDropdown(id) {
    const menu = document.getElementById(id);
    const isOpen = !menu.hidden;
    closeAllDropdowns();
    if (!isOpen) {
        menu.removeAttribute("hidden");
        menu.closest(".nav-dropdown").classList.add("open");
    }
}

function closeAllDropdowns() {
    document.querySelectorAll(".dropdown-menu").forEach((m) => {
        m.setAttribute("hidden", "");
        m.closest(".nav-dropdown")?.classList.remove("open");
    });
}

document.addEventListener("click", (e) => {
    if (!e.target.closest(".nav-dropdown")) {
        closeAllDropdowns();
    }
});

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

// Enable Start Copy button when all 5 selects are filled
const COPY_SELECTS = ["source_conn", "source_db", "source_table", "dest_conn", "dest_db"];

function checkCopyReady() {
    const btn = document.getElementById("btn-start-copy");
    if (!btn) return;
    const allFilled = COPY_SELECTS.every((name) => {
        const el = document.querySelector(`[name="${name}"]`);
        return el && el.value;
    });
    btn.disabled = !allFilled;
}

document.addEventListener("change", checkCopyReady);
document.addEventListener("htmx:afterSwap", checkCopyReady);

// Exclude source connection from destination dropdown
document.addEventListener("change", (e) => {
    if (e.target.name !== "source_conn") return;

    const destSelect = document.querySelector("[name='dest_conn']");
    if (!destSelect) return;

    // re-enable any previously excluded option
    destSelect.querySelectorAll("option[disabled]").forEach((o) => (o.disabled = false));

    const chosen = e.target.value;
    if (!chosen) return;

    const mirror = destSelect.querySelector(`option[value="${CSS.escape(chosen)}"]`);
    if (!mirror) return;

    mirror.disabled = true;

    if (destSelect.value === chosen) {
        destSelect.value = "";
        const wrap = document.getElementById("dest-db-wrap");
        if (wrap) wrap.innerHTML = "";
    }
});

// HTMX
document.addEventListener("DOMContentLoaded", () => {
    document.body.addEventListener("htmx:beforeRequest", () => {
        document.body.classList.add("loading");
    });

    document.body.addEventListener("htmx:afterRequest", () => {
        document.body.classList.remove("loading");
    });
});
