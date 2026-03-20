export default function AuthLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex min-h-screen items-center justify-center bg-background px-4">
      <div className="w-full max-w-sm">
        <div className="mb-10 text-center">
          <div className="mx-auto mb-4">
            <svg className="h-8 w-8 mx-auto" viewBox="0 0 24 24" fill="currentColor">
              <path d="M12 2L2 22h20L12 2z" />
            </svg>
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
