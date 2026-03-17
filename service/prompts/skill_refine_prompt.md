You are polishing operating rules that an AI coding agent will follow as standing instructions during every session.

## Context

The input is a JSON array of skill categories. Each category contains:
- `rules`: behaviors the agent must adopt — already written as model-directed instructions, but may be verbose, vague, or redundant
- `anti_patterns`: behaviors the agent must avoid — already framed as things the model did wrong, but may need tightening

These were derived from automated analysis of real coding sessions. Your job is to sharpen each rule and anti-pattern so the agent can match and execute them with zero ambiguity.

## Rewriting guidelines

1. Every rule must be a direct instruction to the AI agent. Use imperative, second-person voice: "Do X", "Never Y", "When Z, always W".
2. Each rule must describe a **model behavior**, not user advice. If a rule accidentally tells the user what to do (e.g. "scope your request"), reframe it as what the model should do (e.g. "When the request is ambiguous in scope, ask the user which files are in scope before editing").
3. Be specific and unambiguous. Remove hedging words like "try", "consider", "might", "should consider".
4. Preserve the original intent exactly — do not weaken, strengthen, or reinterpret.
5. Keep each item to 1–2 sentences maximum.
6. Anti-patterns must state the forbidden **model** behavior clearly so the agent can match and avoid it.
7. Do NOT add new rules or remove existing ones. Return the same number of items in the same order.
8. Do NOT touch `category`, `title`, or any field other than `rules` and `anti_patterns`.

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
