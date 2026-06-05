// HTMX configuration
document.addEventListener("DOMContentLoaded", () => {
    document.body.addEventListener("htmx:beforeRequest", () => {
        document.body.classList.add("loading");
    });

    document.body.addEventListener("htmx:afterRequest", () => {
        document.body.classList.remove("loading");
    });
});
