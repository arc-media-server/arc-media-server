# Keep typing uninterrupted in Search

Search waits 350 ms after typing pauses. Its own URL updates keep the draft text in the input,
including a space typed before starting the next word. Back, Forward and clearing Search
still update the results and the input.

The browser regression types two words with a pause between them and checks the requests,
input focus and history. Existing HTTP logging covers Search.
