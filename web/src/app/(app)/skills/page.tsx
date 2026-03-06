"use client";

import { useState, useMemo } from "react";
import { useRouter } from "next/navigation";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { clientFetch } from "@/lib/api";
import type { Skill } from "@/lib/types";

const SKILLS: Skill[] = [
  {
    id: "code-review",
    name: "Code Review",
    description: "Analyze code for bugs, security issues, and best practices",
    category: "Development",
    agent_file: "You are a code review assistant. Analyze the provided code for bugs, security vulnerabilities, and suggest improvements following best practices.",
  },
  {
    id: "web-scraper",
    name: "Web Scraper",
    description: "Extract structured data from websites",
    category: "Data",
    agent_file: "You are a web scraping assistant. Help the user extract structured data from websites. Write scripts to collect and format data.",
  },
  {
    id: "sql-assistant",
    name: "SQL Assistant",
    description: "Write and optimize SQL queries",
    category: "Data",
    agent_file: "You are a SQL expert. Help write, optimize, and debug SQL queries. Explain query plans and suggest indexes.",
  },
  {
    id: "doc-writer",
    name: "Doc Writer",
    description: "Generate documentation from code or specs",
    category: "Writing",
    agent_file: "You are a technical documentation writer. Generate clear, comprehensive documentation from code, APIs, or specifications.",
  },
  {
    id: "test-generator",
    name: "Test Generator",
    description: "Generate unit and integration tests for your code",
    category: "Development",
    agent_file: "You are a test generation assistant. Analyze code and generate comprehensive unit and integration tests with good coverage.",
  },
  {
    id: "devops-helper",
    name: "DevOps Helper",
    description: "Docker, K8s configs, CI/CD pipelines",
    category: "Infrastructure",
    agent_file: "You are a DevOps assistant. Help with Docker, Kubernetes configurations, CI/CD pipelines, and infrastructure automation.",
  },
  {
    id: "api-designer",
    name: "API Designer",
    description: "Design RESTful or GraphQL API schemas",
    category: "Development",
    agent_file: "You are an API design assistant. Help design RESTful or GraphQL APIs with proper resource modeling, authentication, and documentation.",
  },
  {
    id: "data-analyst",
    name: "Data Analyst",
    description: "Analyze datasets and generate insights",
    category: "Data",
    agent_file: "You are a data analysis assistant. Analyze datasets, generate visualizations, and provide statistical insights and summaries.",
  },
  {
    id: "shell-assistant",
    name: "Shell Assistant",
    description: "Write bash scripts and CLI commands",
    category: "Infrastructure",
    agent_file: "You are a shell scripting assistant. Help write bash scripts, CLI commands, and automate system administration tasks.",
  },
];

const CATEGORIES = ["All", ...Array.from(new Set(SKILLS.map((s) => s.category)))];

export default function SkillsPage() {
  const router = useRouter();
  const [search, setSearch] = useState("");
  const [category, setCategory] = useState("All");
  const [runningSkill, setRunningSkill] = useState<string | null>(null);

  const filtered = useMemo(() => {
    return SKILLS.filter((s) => {
      const matchesCategory = category === "All" || s.category === category;
      const matchesSearch =
        !search ||
        s.name.toLowerCase().includes(search.toLowerCase()) ||
        s.description.toLowerCase().includes(search.toLowerCase());
      return matchesCategory && matchesSearch;
    });
  }, [search, category]);

  const runSkill = async (skill: Skill) => {
    setRunningSkill(skill.id);
    try {
      const res = await clientFetch("/api/runs", {
        method: "POST",
        body: JSON.stringify({
          name: skill.name,
          agent_file: skill.agent_file,
        }),
      });
      const data = await res.json();
      if (data.id) {
        router.push(`/runs/${data.id}`);
      }
    } catch {
      // silently fail
    } finally {
      setRunningSkill(null);
    }
  };

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Skills</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Pre-built agent skills ready to run
        </p>
      </div>

      {/* Filters */}
      <div className="flex flex-col sm:flex-row gap-3">
        <Input
          placeholder="Search skills..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="sm:max-w-xs"
        />
        <div className="flex gap-2 flex-wrap">
          {CATEGORIES.map((cat) => (
            <Button
              key={cat}
              variant={category === cat ? "default" : "outline"}
              size="sm"
              onClick={() => setCategory(cat)}
            >
              {cat}
            </Button>
          ))}
        </div>
      </div>

      {/* Grid */}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {filtered.map((skill) => (
          <Card key={skill.id} className="flex flex-col">
            <CardHeader className="pb-2">
              <div className="flex items-start justify-between">
                <CardTitle className="text-sm font-semibold">
                  {skill.name}
                </CardTitle>
                <Badge variant="secondary" className="text-[10px]">
                  {skill.category}
                </Badge>
              </div>
            </CardHeader>
            <CardContent className="flex flex-1 flex-col justify-between gap-3">
              <p className="text-sm text-muted-foreground">
                {skill.description}
              </p>
              <Button
                size="sm"
                variant="outline"
                onClick={() => runSkill(skill)}
                disabled={runningSkill === skill.id}
              >
                {runningSkill === skill.id ? "Starting..." : "Run"}
              </Button>
            </CardContent>
          </Card>
        ))}
      </div>

      {filtered.length === 0 && (
        <div className="py-12 text-center text-sm text-muted-foreground">
          No skills match your search
        </div>
      )}
    </div>
  );
}
