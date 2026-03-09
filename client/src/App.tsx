import { useEffect, useState } from 'react';

function App() {
  const [query, setQuery] = useState<string>('');
  const [files, setFiles] = useState<string[]>([]);

  useEffect(() => {
    fetch('/images')
      .then((res) => res.json())
      .then((res) => setFiles(res.files));
  }, []);

  useEffect(() => {
    if (!query) return;
    const timeout = setTimeout(
      () =>
        fetch('/copy', {
          method: 'POST',
          body: JSON.stringify({ query }),
          headers: { 'content-type': 'application/json' },
        }).then((res) => res.ok),
      1000,
    );
    return () => clearTimeout(timeout);
  }, [query]);

  return (
    <div className="m-10">
      <textarea
        className="mb-5 border-input placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-ring/50 aria-invalid:ring-destructive/20 aria-invalid:border-destructive flex field-sizing-content min-h-16 w-full rounded-md border bg-transparent px-3 py-2 text-base shadow-xs transition-[color,box-shadow] outline-none focus-visible:ring-[3px] disabled:cursor-not-allowed border-gray-500 disabled:opacity-50 md:text-sm"
        data-slot="textarea"
        value={query}
        rows={5}
        onChange={(e) => setQuery(e.target.value)}
      />

      {files.map((src, index) => (
        <img key={index} src={src} width={800} height={600} />
      ))}
    </div>
  );
}

export default App;
