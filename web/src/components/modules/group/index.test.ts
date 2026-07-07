import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const source = fs.readFileSync(path.join(__dirname, 'index.tsx'), 'utf8');

function countOccurrences(content: string, needle: string) {
    return content.split(needle).length - 1;
}

test('group empty state keeps a single explanatory section', () => {
    assert.equal(
        countOccurrences(source, "t('emptyState.description')"),
        1,
        'group empty state should not render duplicate empty-state descriptions',
    );
});

test('group page toolbar area exposes the grouped view toggle near filter controls', () => {
    const groupedToggleNeedle = "groupedRoute";
    assert.ok(
        source.includes(groupedToggleNeedle) || source.includes('route-model') || source.includes('grouped-view'),
        'group page toolbar should include a grouped view toggle control near the filter UI',
    );
});
