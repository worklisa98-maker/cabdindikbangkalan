const nextConfig = {
  async redirects() {
    return [
      {
        source: "/siswa-perguruan-tinggi",
        destination: "/siswa/status/diterima-perguruan-tinggi",
        permanent: false,
      },
    ];
  },
};

export default nextConfig;
