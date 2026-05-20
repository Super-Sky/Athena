---
name: webapp-testing
description: Use when testing local web applications with Playwright and browser inspection.
---

# Web Application Testing

To test local web applications, write native Python Playwright scripts.

If the app is not already running, start it with the repository's normal local dev command before running the script.

## Decision Tree: Choosing Your Approach

```
User task → Is it static HTML?
    ├─ Yes → Read HTML file directly to identify selectors
    │         ├─ Success → Write Playwright script using selectors
    │         └─ Fails/Incomplete → Treat as dynamic (below)
    │
    └─ No (dynamic webapp) → Is the server already running?
        ├─ No → Start the app, then run the automation script
        └─ Yes → Reconnaissance-then-action:
            1. Navigate and wait for networkidle
            2. Take screenshot or inspect DOM
            3. Identify selectors from rendered state
            4. Execute actions with discovered selectors
```

## Reconnaissance-Then-Action Pattern

1. **Inspect rendered DOM**:
   ```python
   page.screenshot(path='/tmp/inspect.png', full_page=True)
   content = page.content()
   page.locator('button').all()
   ```

2. **Identify selectors** from inspection results.

3. **Execute actions** using discovered selectors.

## Common Pitfall

- Do not inspect the DOM before waiting for `networkidle` on dynamic apps.
- Do wait for `page.wait_for_load_state('networkidle')` before inspection.

## Best Practices

- Use synchronous Playwright scripts when possible.
- Always close the browser when done.
- Use descriptive selectors: `text=`, `role=`, CSS selectors, or IDs.
- Add appropriate waits: `page.wait_for_selector()` or `page.wait_for_timeout()`.
- When the browser is useful, capture screenshots and console output to verify behavior.
