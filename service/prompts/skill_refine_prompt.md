You are rewriting behavioral rules so an AI coding agent can follow them as direct operating instructions.

## Context

The input is a JSON array of skill categories. Each category contains:
- `rules`: things the agent should do (currently phrased as advice for a human developer)
- `anti_patterns`: things the agent must not do (currently phrased as observations about bad habits)

These were extracted from session analysis reports written for humans. Your job is to rewrite every rule and anti-pattern into a form that an AI agent can execute literally.

## Rewriting guidelines

1. Use imperative, second-person voice addressed to the agent: "Do X", "Never Y", "When Z, always W".
2. Be specific and unambiguous. Remove hedging words like "try", "consider", "might", "should consider".
3. Preserve the original intent exactly — do not weaken, strengthen, or reinterpret.
4. Keep each item to 1–2 sentences maximum.
5. Anti-patterns must state the forbidden behavior clearly so the agent can match and avoid it.
6. Do NOT add new rules or remove existing ones. Return the same number of items in the same order.
7. Do NOT touch `category`, `title`, or any field other than `rules` and `anti_patterns`.

## Input

{{.CategoriesJSON}}

## Output format

Return valid JSON only. No markdown fences, no commentary.

Return an array with the same length and order as the input. Each element must have:

```
{
  "category": "<unchanged>",
  "rules": ["rewritten rule 1", ...],
  "anti_patterns": ["rewritten anti-pattern 1", ...]
}
```
