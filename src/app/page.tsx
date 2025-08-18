"use client";

import Image from "next/image";
import { useEffect, useRef, useState } from "react";
import { Button } from "../components/ui/button";
import { Input } from "../components/ui/input";

export default function Home() {
  const input = useRef<HTMLInputElement>(null);
  const [files, setFiles] = useState<string[]>([]);

  useEffect(() => {
    fetch("/api/img")
      .then((res) => res.json())
      .then((res) => setFiles(res.files));
  }, []);

  return (
    <div className="flex flex-col p-8">
      <div className="flex">
        <div className="flex w-full max-w-sm items-center gap-2">
          <Input ref={input} type="text" placeholder="Enter text here" />
          <Button
            type="button"
            variant="default"
            className="cursor-pointer"
            onClick={() =>
              fetch("/api/copy", {
                method: "POST",
                body: JSON.stringify({ text: input.current?.value || "" }),
              }).then((res) => res.json())
            }
          >
            Clipboard
          </Button>
        </div>
      </div>
      {files.map((src, index) => (
        <Image
          key={index}
          src={src}
          alt="Next.js logo"
          width={800}
          height={600}
          priority
        />
      ))}
    </div>
  );
}
