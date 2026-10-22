// Diagnostic review only: these assertions document current gaps, not desired behaviour.
import { defineConfig } from '@playwright/test';
import base from './playwright.config';
export default defineConfig({ ...base, testDir: './review', outputDir: './test-results/ui-review', reporter: [['list']] });
