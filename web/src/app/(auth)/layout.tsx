export default function AuthLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex min-h-screen items-center justify-center bg-background px-4">
      <div className="w-full max-w-sm">
        <div className="mb-8 text-center">
          <div className="mx-auto mb-3 flex h-10 w-10 items-center justify-center rounded-lg bg-primary text-primary-foreground text-sm font-bold">
            A
          </div>
          <h1 className="text-xl font-semibold tracking-tight">ABox</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            AI Agents, One Click Away
          </p>
        </div>
        {children}
      </div>
    </div>
  );
}
