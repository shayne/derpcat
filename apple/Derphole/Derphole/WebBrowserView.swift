import Combine
import SwiftUI
import WebKit

struct WebBrowserView: View {
    let url: URL
    let route: WebTunnelState.Route
    let onDisconnect: () -> Void

    @Environment(\.dismiss) private var dismiss
    @StateObject private var browserState = WebBrowserState()

    var body: some View {
        VStack(spacing: 0) {
            browserChrome
            Divider()
            WebViewRepresentable(url: url, browserState: browserState)
                .ignoresSafeArea(edges: .bottom)
                .accessibilityIdentifier("webBrowserView")
        }
        .navigationBarTitleDisplayMode(.inline)
        .navigationTitle("Web")
        .toolbar {
            ToolbarItem(placement: .topBarTrailing) {
                Button(role: .destructive) {
                    onDisconnect()
                    dismiss()
                } label: {
                    Label("Disconnect", systemImage: "xmark.circle")
                }
                .accessibilityIdentifier("webDisconnectButton")
            }
        }
    }

    private var browserChrome: some View {
        HStack(spacing: 10) {
            Button {
                browserState.goBack()
            } label: {
                Image(systemName: "chevron.left")
            }
            .disabled(!browserState.canGoBack)
            .accessibilityLabel("Back")

            Button {
                browserState.goForward()
            } label: {
                Image(systemName: "chevron.right")
            }
            .disabled(!browserState.canGoForward)
            .accessibilityLabel("Forward")

            Button {
                browserState.reload()
            } label: {
                Image(systemName: "arrow.clockwise")
            }
            .accessibilityLabel("Reload")

            Text(browserState.displayAddress ?? url.absoluteString)
                .font(.caption.monospaced())
                .lineLimit(1)
                .truncationMode(.middle)
                .padding(.horizontal, 10)
                .padding(.vertical, 7)
                .frame(maxWidth: .infinity, alignment: .leading)
                .background(Color(.secondarySystemBackground), in: Capsule())

            Text(route.label)
                .font(.caption.weight(.semibold))
                .padding(.horizontal, 9)
                .padding(.vertical, 6)
                .background(routeBackground, in: Capsule())
                .accessibilityLabel("Route \(route.label)")
        }
        .buttonStyle(.borderless)
        .padding(.horizontal, 12)
        .padding(.vertical, 8)
        .background(.bar)
    }

    private var routeBackground: Color {
        switch route {
        case .unknown:
            return .gray.opacity(0.16)
        case .relay:
            return .orange.opacity(0.18)
        case .direct:
            return .green.opacity(0.18)
        }
    }
}

final class WebBrowserState: ObservableObject {
    @Published var canGoBack = false
    @Published var canGoForward = false
    @Published var displayAddress: String?

    weak var webView: WebViewControlling?

    func goBack() {
        _ = webView?.goBack()
    }

    func goForward() {
        _ = webView?.goForward()
    }

    func reload() {
        _ = webView?.reload()
    }

    func update(canGoBack: Bool, canGoForward: Bool, url: URL?) {
        self.canGoBack = canGoBack
        self.canGoForward = canGoForward
        displayAddress = url?.absoluteString
    }
}

protocol WebViewControlling: AnyObject {
    func goBack() -> WKNavigation?
    func goForward() -> WKNavigation?
    func reload() -> WKNavigation?
}
